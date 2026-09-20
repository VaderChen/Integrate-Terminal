#!/usr/bin/env python3
"""Read-only signing preflight and explicitly requested, isolated ad-hoc sandbox probes.

No certificate import, signing with an Apple identity, Keychain search-list changes,
application installation, real application startup, or App Store submission occurs.
"""
import argparse
import datetime as dt
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import re
import shutil
import subprocess
import sys
import tempfile
import uuid

PROJECT = Path(__file__).resolve().parents[2]
SOURCE = Path(__file__).with_name("sandbox_probe.m")
BUNDLE_ID = "com.vader.integterm"
sys.path.insert(0, str(PROJECT / "scripts"))
from appstore_config import resolve_entitlements


def run(command, **kwargs):
    return subprocess.run(command, check=True, capture_output=True, **kwargs)


def parse_identities(output):
    identities = []
    for fingerprint, description in re.findall(r'\b([0-9A-F]{40}) "([^"]+)"', output):
        team = re.search(r"\(([^)]+)\)$", description)
        identities.append({"sha1": fingerprint, "kind": description.split(":", 1)[0], "team": team.group(1) if team else None})
    return identities


def utc(value):
    return value.replace(tzinfo=dt.timezone.utc) if value.tzinfo is None else value.astimezone(dt.timezone.utc)


def summarize_profile(profile, entitlements, identities, now=None):
    now = now or dt.datetime.now(dt.timezone.utc)
    allowed = profile.get("Entitlements", {})
    app_id = allowed.get("com.apple.application-identifier", allowed.get("application-identifier", ""))
    target_id = entitlements.get("com.apple.application-identifier", "")
    team = entitlements.get("com.apple.developer.team-identifier", "")
    certificates = {hashlib.sha1(cert).hexdigest().upper() for cert in profile.get("DeveloperCertificates", [])}
    matching = [identity for identity in identities if identity["sha1"] in certificates]
    groups = entitlements.get("keychain-access-groups", [])
    authorized_groups = allowed.get("keychain-access-groups", [])
    expiration = profile.get("ExpirationDate")
    creation = profile.get("CreationDate")
    checks = {
        "platform_macos": "OSX" in profile.get("Platform", []),
        "profile_not_expired": bool(expiration and utc(expiration) > now),
        "profile_already_valid": bool(creation and utc(creation) <= now),
        "bundle_identifier_matches": target_id.endswith(f".{BUNDLE_ID}") and fnmatch.fnmatchcase(target_id, app_id),
        "team_matches": bool(team) and team in profile.get("TeamIdentifier", []),
        "matching_private_key_identity_available": bool(matching),
        "keychain_groups_authorized": bool(groups) and all(any(fnmatch.fnmatchcase(group, pattern) for pattern in authorized_groups) for group in groups),
        "keychain_groups_concrete": all("*" not in group and "?" not in group for group in groups),
        "app_sandbox_enabled": entitlements.get("com.apple.security.app-sandbox") is True,
        "network_server_enabled": entitlements.get("com.apple.security.network.server") is True,
    }
    return {
        "name": profile.get("Name"), "uuid": profile.get("UUID"),
        "platform": profile.get("Platform", []), "team": profile.get("TeamIdentifier", []),
        "application_identifier": app_id,
        "expires_utc": utc(expiration).isoformat() if expiration else None,
        "matching_signing_identity_kinds": [item["kind"] for item in matching],
        "keychain_groups": groups, "profile_authorized_keychain_patterns": authorized_groups,
        "checks": checks, "ready_for_signing_review": all(checks.values()),
        "apple_signed_runtime_verified": False,
    }


def preflight(profile_path, entitlement_path):
    identities = parse_identities(run(["/usr/bin/security", "find-identity", "-v", "-p", "codesigning"], text=True).stdout)
    profile = plistlib.loads(run(["/usr/bin/security", "cms", "-D", "-i", str(profile_path)]).stdout)
    entitlements = plistlib.loads(entitlement_path.read_bytes())
    if entitlements.get("com.apple.application-identifier") == "${APP_IDENTIFIER}":
        entitlements = resolve_entitlements(profile, entitlements, BUNDLE_ID)
    result = summarize_profile(profile, entitlements, identities)
    result["profile_path"] = str(profile_path)
    result["entitlements_path"] = str(entitlement_path)
    return result


def keychain_clean(result):
    for backend in ("classic", "data_protection"):
        item = result.get(backend, {})
        if item.get("add_status") == 0 and (item.get("delete_status") != 0 or item.get("after_delete_status") != -25300):
            return False
    return True


def validate_probes(results):
    failures = []
    for result in results:
        mode = result["mode"]
        output = result.get("output", {})
        if result.get("exit_code") != 0:
            failures.append(f"{mode}: probe process failed")
            continue
        if output.get("home_is_probe_container") is not (mode != "plain-adhoc"):
            failures.append(f"{mode}: unexpected sandbox container placement")
        classic = output.get("classic", {})
        if any(classic.get(key) != 0 for key in ("add_status", "read_status", "update_status", "updated_read_status", "delete_status")) or not classic.get("read_matches") or not classic.get("updated_read_matches") or classic.get("after_delete_status") != -25300:
            failures.append(f"{mode}: classic Keychain CRUD/cleanup failed; the probe does not unlock Keychain or prompt")
        if output.get("data_protection", {}).get("add_status") != -34018:
            failures.append(f"{mode}: unexpected Data Protection result for an ad-hoc identity without an application identifier")
        network = output.get("network", {})
        if mode == "sandbox-no-server":
            if network.get("bind_result") != -1 or network.get("bind_errno") != 1:
                failures.append(f"{mode}: expected sandbox to deny loopback bind with EPERM")
        elif network.get("bind_result") != 0 or network.get("listen_result") != 0:
            failures.append(f"{mode}: loopback bind/listen failed")
    return failures


def adhoc_probes():
    staging = Path(tempfile.mkdtemp(prefix="integterm-sandbox-probe-", dir="/tmp"))
    results = []
    keep_staging = False
    try:
        executable = staging / "compiled-probe"
        run(["/usr/bin/xcrun", "clang", "-fobjc-arc", "-mmacosx-version-min=12.0", "-Wno-deprecated-declarations", "-framework", "Foundation", "-framework", "Security", str(SOURCE), "-o", str(executable)])
        for mode in ("plain-adhoc", "sandbox-no-server", "sandbox-with-server"):
            run_id = str(uuid.uuid4())
            bundle_id = f"com.vader.integterm.sandbox-probe.{run_id}"
            app = staging / f"{mode}.app"
            macos = app / "Contents/MacOS"
            macos.mkdir(parents=True)
            shutil.copy2(executable, macos / "SandboxProbe")
            (app / "Contents/Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": bundle_id, "CFBundleExecutable": "SandboxProbe", "CFBundleName": "IntegTERM Sandbox Probe", "CFBundleVersion": "1", "CFBundlePackageType": "APPL"}))
            entitlements = {} if mode == "plain-adhoc" else {"com.apple.security.app-sandbox": True, "com.apple.security.network.client": True, "com.apple.security.network.server": mode == "sandbox-with-server"}
            entitlements_path = staging / f"{mode}.plist"
            entitlements_path.write_bytes(plistlib.dumps(entitlements))
            run(["/usr/bin/codesign", "--force", "--sign", "-", "--entitlements", str(entitlements_path), str(app)])
            run(["/usr/bin/codesign", "--verify", "--deep", "--strict", str(app)])
            # Only remove a container whose unpredictable ID was created by this invocation.
            container = Path.home() / "Library/Containers" / bundle_id
            container_existed = container.exists()
            try:
                completed = subprocess.run([str(macos / "SandboxProbe"), run_id], capture_output=True, text=True, timeout=25)
                output = json.loads(completed.stdout) if completed.stdout.strip().startswith("{") else {}
                result = {"mode": mode, "signing": "ad-hoc", "exit_code": completed.returncode, "output": output}
                if completed.returncode or not keychain_clean(output):
                    keep_staging = True
                    result["cleanup_review_required"] = True
                    result["fake_item_service"] = f"com.vader.integterm.sandbox-probe.{run_id}"
            except subprocess.TimeoutExpired:
                keep_staging = True
                result = {"mode": mode, "signing": "ad-hoc", "exit_code": "timeout", "cleanup_review_required": True, "fake_item_service": f"com.vader.integterm.sandbox-probe.{run_id}"}
            if container.exists() and not container_existed:
                try:
                    shutil.rmtree(container)
                    result["container_cleanup"] = "removed"
                except OSError as error:
                    result["container_cleanup"] = "OS-protected container metadata remains"
                    result["container_cleanup_errno"] = error.errno
                    result["remaining_container"] = str(container)
            results.append(result)
        failures = validate_probes(results)
        return {"apple_signed_runtime_verified": False, "results": results, "failures": failures, "passed": not failures, **({"retained_staging_for_cleanup": str(staging)} if keep_staging else {})}
    finally:
        if not keep_staging:
            shutil.rmtree(staging)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("preflight", "adhoc", "all"), default="preflight")
    parser.add_argument("--profile", type=Path, default=PROJECT / "cert/IntegTerm.provisionprofile")
    parser.add_argument("--entitlements", type=Path, default=PROJECT / "entitlements.plist")
    args = parser.parse_args()
    if platform.system() != "Darwin":
        parser.error("This diagnostic requires macOS and the Apple Security framework")
    result = {"time_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "macos": platform.mac_ver()[0], "machine": platform.machine()}
    try:
        if args.mode in ("preflight", "all"):
            result["preflight"] = preflight(args.profile, args.entitlements)
        if args.mode in ("adhoc", "all"):
            result["adhoc_probes"] = adhoc_probes()
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        # No subprocess input, certificate bytes, Keychain data, or profile contents are printed.
        result["error"] = f"{type(error).__name__}: diagnostic step failed"
    print(json.dumps(result, ensure_ascii=False, indent=2))
    valid = "error" not in result and result.get("preflight", {}).get("ready_for_signing_review", True) and result.get("adhoc_probes", {}).get("passed", True)
    return 0 if valid else 1


if __name__ == "__main__":
    raise SystemExit(main())
