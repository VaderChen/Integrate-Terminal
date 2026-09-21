#!/usr/bin/env python3
"""Opt-in Apple-identity signing of a UUID sandbox diagnostic, never the product app.

The existing production profile has an exact App ID. Keeping a UUID bundle protects
its existing container, but the OS may reject that mismatch. Static codesign success
is never reported as runtime or production App Store validation.
"""
import argparse
import datetime as dt
import json
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tempfile
import uuid

from probe_sandbox import PROJECT, SOURCE, BUNDLE_ID, keychain_clean, parse_identities, run, summarize_profile, resolve_entitlements


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--identity-sha1", required=True, help="Explicit SHA-1 of an existing profile-matching signing identity; no import or Keychain settings change")
    parser.add_argument("--profile", type=Path, default=PROJECT / "cert/IntegTerm.provisionprofile")
    parser.add_argument("--entitlements", type=Path, default=PROJECT / "entitlements.plist")
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9A-Fa-f]{40}", args.identity_sha1):
        parser.error("--identity-sha1 must contain exactly 40 hexadecimal characters")
    identity_sha1 = args.identity_sha1.upper()
    identities = parse_identities(run(["/usr/bin/security", "find-identity", "-v", "-p", "codesigning"], text=True).stdout)
    chosen = [item for item in identities if item["sha1"] == identity_sha1]
    profile = plistlib.loads(run(["/usr/bin/security", "cms", "-D", "-i", str(args.profile)]).stdout)
    entitlements = plistlib.loads(args.entitlements.read_bytes())
    if entitlements.get("com.apple.application-identifier") == "${APP_IDENTIFIER}":
        entitlements = resolve_entitlements(profile, entitlements, BUNDLE_ID)
    preflight = summarize_profile(profile, entitlements, chosen)
    if not preflight["ready_for_signing_review"]:
        print(json.dumps({"preflight": preflight, "error": "Explicit identity/profile/entitlements preflight failed"}, indent=2))
        return 1
    run_id = str(uuid.uuid4())
    bundle_id = f"com.vader.integterm.sandbox-probe.{run_id}"
    result = {"time_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "bundle_id": bundle_id,
              "signing_identity_kind": chosen[0]["kind"], "team": entitlements["com.apple.developer.team-identifier"],
              "uses_production_bundle_identifier": False, "profile_matches_probe_bundle_identifier": False,
              "production_appstore_runtime_verified": False}
    staging = Path(tempfile.mkdtemp(prefix="integterm-signed-probe-", dir="/tmp"))
    container = Path.home() / "Library/Containers" / bundle_id
    container_existed = container.exists()
    keep_staging = False
    try:
        app = staging / "SandboxProbe.app"
        executable = app / "Contents/MacOS/SandboxProbe"
        executable.parent.mkdir(parents=True)
        run(["/usr/bin/xcrun", "clang", "-fobjc-arc", "-mmacosx-version-min=13.0", "-Wno-deprecated-declarations", "-framework", "Foundation", "-framework", "Security", str(SOURCE), "-o", str(executable)])
        (app / "Contents/Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": bundle_id, "CFBundleExecutable": "SandboxProbe", "CFBundleName": "IntegTERM Signed Sandbox Probe", "CFBundleVersion": "1", "CFBundlePackageType": "APPL"}))
        shutil.copy2(args.profile, app / "Contents/embedded.provisionprofile")
        entitlement_path = staging / "entitlements.plist"
        entitlement_path.write_bytes(plistlib.dumps(entitlements))
        signed = subprocess.run(["/usr/bin/codesign", "--force", "--sign", identity_sha1, "--options", "runtime", "--timestamp=none", "--entitlements", str(entitlement_path), str(app)], capture_output=True, text=True, timeout=30)
        result["sign_exit_code"] = signed.returncode
        if signed.returncode == 0:
            verify = subprocess.run(["/usr/bin/codesign", "--verify", "--deep", "--strict", str(app)], capture_output=True, text=True, timeout=30)
            result["verify_exit_code"] = verify.returncode
            if verify.returncode == 0:
                executed = subprocess.run([str(executable), run_id], capture_output=True, text=True, timeout=25)
                result["launch_exit_code"] = executed.returncode
                result["probe_output"] = json.loads(executed.stdout) if executed.stdout.strip().startswith("{") else None
                result["apple_signed_probe_launched"] = executed.returncode == 0 and bool(result["probe_output"])
                output = result["probe_output"] or {}
                classic = output.get("classic", {})
                network = output.get("network", {})
                result["apple_signed_probe_runtime_verified"] = bool(result["apple_signed_probe_launched"] and output.get("home_is_probe_container") and network.get("bind_result") == 0 and network.get("listen_result") == 0 and all(classic.get(key) == 0 for key in ("add_status", "read_status", "update_status", "updated_read_status", "delete_status")) and classic.get("read_matches") and classic.get("updated_read_matches") and keychain_clean(output))
                if executed.returncode or not keychain_clean(output):
                    keep_staging = True
                    result["cleanup_review_required"] = True
                    result["fake_item_service"] = bundle_id
                if executed.returncode:
                    # Only this unpredictable probe ID's launch logs; no unrelated user logs.
                    logs = subprocess.run(["/usr/bin/log", "show", "--last", "2m", "--style", "compact", "--info", "--predicate", f'(process == "amfid" OR process == "taskgated-helper") AND (eventMessage CONTAINS "{bundle_id}" OR eventMessage CONTAINS "{staging.name}")'], capture_output=True, text=True, timeout=30)
                    result["launch_diagnostics"] = logs.stdout.strip()
    except subprocess.TimeoutExpired:
        keep_staging = True
        result["error"] = "A probe step timed out; runtime validation incomplete"
        result["cleanup_review_required"] = True
        result["fake_item_service"] = bundle_id
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        keep_staging = True
        result["error"] = f"{type(error).__name__}: diagnostic step failed"
        result["cleanup_review_required"] = True
        result["fake_item_service"] = bundle_id
    finally:
        if container.exists() and not container_existed:
            try:
                shutil.rmtree(container)
                result["container_cleanup"] = "removed"
            except OSError as error:
                result["container_cleanup"] = "OS-protected probe container metadata remains"
                result["container_cleanup_errno"] = error.errno
                result["remaining_container"] = str(container)
        if keep_staging:
            result["retained_staging_for_cleanup"] = str(staging)
        else:
            shutil.rmtree(staging)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result.get("apple_signed_probe_runtime_verified") else 1


if __name__ == "__main__":
    raise SystemExit(main())
