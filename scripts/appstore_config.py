#!/usr/bin/env python3
"""Resolve local App Store signing settings and profile-authorized entitlements."""
import argparse
import datetime as dt
import fnmatch
import hashlib
import json
from pathlib import Path
import plistlib
import re
import sys

CONFIG_KEYS = {"TEAM_ID", "APP_CERT", "INSTALLER_CERT", "BUNDLE_ID", "PROVISION_PROFILE_PATH", "SIGNING_KEYCHAIN", "CERT_DIR"}


def config_value(path, key):
    if key not in CONFIG_KEYS:
        raise ValueError("Unsupported signing setting")
    if not path.exists():
        return ""
    value = json.loads(path.read_text()).get(key, "")
    if not isinstance(value, str) or any(char in value for char in "\n\r\0"):
        raise ValueError("Signing settings must be single-line strings")
    return value


def allowed_identifier(value, patterns):
    # Provisioning profiles may authorize a prefix wildcard; the application
    # itself always receives one concrete identifier, never a wildcard group.
    return any(isinstance(pattern, str) and fnmatch.fnmatchcase(value, pattern) for pattern in patterns)


def resolve_entitlements(profile, template, bundle_id, expected_team=""):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9.-]*", bundle_id):
        raise ValueError("Invalid bundle identifier")
    teams = profile.get("TeamIdentifier", [])
    if len(teams) != 1 or not isinstance(teams[0], str) or not re.fullmatch(r"[A-Z0-9]{10}", teams[0]):
        raise ValueError("Profile must identify one valid signing team")
    team = teams[0]
    if expected_team and team != expected_team:
        raise ValueError("Configured signing team does not match the provisioning profile")
    allowed = profile.get("Entitlements", {})
    profile_app_id = allowed.get("com.apple.application-identifier", allowed.get("application-identifier", ""))
    prefixes = profile.get("ApplicationIdentifierPrefix", [team])
    candidates = [f"{prefix}.{bundle_id}" for prefix in prefixes if isinstance(prefix, str) and re.fullmatch(r"[A-Z0-9]{10}", prefix)]
    matches = [candidate for candidate in candidates if allowed_identifier(candidate, [profile_app_id])]
    if len(matches) != 1:
        raise ValueError("Provisioning profile does not authorize this bundle identifier")
    app_id = matches[0]
    if not allowed_identifier(app_id, allowed.get("keychain-access-groups", [])):
        raise ValueError("Provisioning profile does not authorize the application's Keychain group")
    if allowed.get("com.apple.developer.team-identifier", team) != team:
        raise ValueError("Provisioning profile contains inconsistent team identifiers")
    entitlements = dict(template)
    entitlements["com.apple.application-identifier"] = app_id
    entitlements["com.apple.developer.team-identifier"] = team
    entitlements["keychain-access-groups"] = [app_id]
    return entitlements


def validate_profile_dates(profile):
    now = dt.datetime.now(dt.timezone.utc)
    if "OSX" not in profile.get("Platform", []):
        raise ValueError("A macOS provisioning profile is required")
    for key, expired in (("ExpirationDate", True), ("CreationDate", False)):
        value = profile.get(key)
        if not isinstance(value, dt.datetime):
            raise ValueError("Provisioning profile is missing validity dates")
        value = value.replace(tzinfo=dt.timezone.utc) if value.tzinfo is None else value.astimezone(dt.timezone.utc)
        if (expired and value <= now) or (not expired and value > now):
            raise ValueError("Provisioning profile is outside its validity period")


def select_identity(output, kind, team, preferred="", profile=None):
    prefixes = {"app": {"Apple Distribution", "3rd Party Mac Developer Application"},
                "installer": {"Mac Installer Distribution", "3rd Party Mac Developer Installer"}}[kind]
    authorized = {hashlib.sha1(cert).hexdigest().upper() for cert in (profile or {}).get("DeveloperCertificates", [])}
    matches = []
    for fingerprint, description in re.findall(r'\b([0-9A-Fa-f]{40}) "([^"\r\n]+)"', output):
        fingerprint = fingerprint.upper()
        if description.split(":", 1)[0] not in prefixes or not description.endswith(f"({team})"):
            continue
        if kind == "app" and fingerprint not in authorized:
            continue
        if preferred and preferred not in (description, fingerprint):
            continue
        matches.append(fingerprint)
    matches = list(dict.fromkeys(matches))
    if len(matches) != 1:
        raise ValueError("Expected one matching signing identity; set APP_CERT or INSTALLER_CERT explicitly")
    return matches[0]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    value = sub.add_parser("value")
    value.add_argument("file", type=Path)
    value.add_argument("key", choices=sorted(CONFIG_KEYS))
    for command in ("prepare", "matches"):
        item = sub.add_parser(command)
        item.add_argument("--profile", type=Path, required=True)
        item.add_argument("--bundle-id", required=True)
        item.add_argument("--team-id", default="")
        if command == "prepare":
            item.add_argument("--template", type=Path, required=True)
            item.add_argument("--output", type=Path, required=True)
            item.add_argument("--app-info", type=Path, required=True)
    identity = sub.add_parser("identity")
    identity.add_argument("--kind", choices=("app", "installer"), required=True)
    identity.add_argument("--team-id", required=True)
    identity.add_argument("--preferred", default="")
    identity.add_argument("--profile", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.command == "value":
            print(config_value(args.file, args.key))
        else:
            profile = plistlib.loads(args.profile.read_bytes())
            if args.command == "identity":
                print(select_identity(sys.stdin.read(), args.kind, args.team_id, args.preferred, profile))
            else:
                validate_profile_dates(profile)
                if args.command == "prepare" and plistlib.loads(args.app_info.read_bytes()).get("CFBundleIdentifier") != args.bundle_id:
                    raise ValueError("Built application bundle identifier does not match the signing configuration")
                template = plistlib.loads(args.template.read_bytes()) if args.command == "prepare" else {}
                entitlements = resolve_entitlements(profile, template, args.bundle_id, args.team_id)
                if args.command == "prepare":
                    args.output.write_bytes(plistlib.dumps(entitlements))
                    args.output.chmod(0o600)
                    print(entitlements["com.apple.developer.team-identifier"])
    except (OSError, ValueError, TypeError, AttributeError, plistlib.InvalidFileException):
        # Diagnostics never echo local signing values, profiles, or identities.
        print("Signing configuration is invalid, unavailable, or does not match the supplied profile/identity.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
