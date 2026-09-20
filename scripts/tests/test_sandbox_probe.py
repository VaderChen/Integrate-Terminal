#!/usr/bin/env python3
"""Pure metadata/diagnostic regressions: no signing, Keychain, or sandbox execution."""
import datetime as dt
import hashlib
import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location("probe_sandbox", Path(__file__).with_name("probe_sandbox.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
TEAM_ID = "TEAM123456"


class SandboxPreflightTests(unittest.TestCase):
    def setUp(self):
        self.now = dt.datetime(2026, 9, 20, tzinfo=dt.timezone.utc)
        self.profile = {"Name": "Fake", "Platform": ["OSX"], "TeamIdentifier": [TEAM_ID], "CreationDate": self.now - dt.timedelta(days=1), "ExpirationDate": self.now + dt.timedelta(days=1), "DeveloperCertificates": [b"fake-public-certificate"], "Entitlements": {"com.apple.application-identifier": f"{TEAM_ID}.{MODULE.BUNDLE_ID}", "keychain-access-groups": [f"{TEAM_ID}.*"]}}
        self.entitlements = {"com.apple.application-identifier": f"{TEAM_ID}.{MODULE.BUNDLE_ID}", "com.apple.developer.team-identifier": TEAM_ID, "keychain-access-groups": [f"{TEAM_ID}.{MODULE.BUNDLE_ID}"], "com.apple.security.app-sandbox": True, "com.apple.security.network.server": True}
        self.identities = [{"kind": "3rd Party Mac Developer Application", "sha1": hashlib.sha1(b"fake-public-certificate").hexdigest().upper(), "team": TEAM_ID}]

    def report(self):
        return MODULE.summarize_profile(self.profile, self.entitlements, self.identities, self.now)

    def test_matching_profile_is_only_signing_preflight_not_runtime_verification(self):
        report = self.report()
        self.assertTrue(report["ready_for_signing_review"])
        self.assertFalse(report["apple_signed_runtime_verified"])

    def test_template_entitlements_are_resolved_from_supplied_profile(self):
        import plistlib
        import tempfile
        from types import SimpleNamespace
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "entitlements.plist"
            path.write_bytes((Path(__file__).resolve().parents[2] / "entitlements.plist").read_bytes())
            identity = '1) ' + self.identities[0]["sha1"] + ' "Apple Distribution: Example Person (TEAM123456)"'
            with patch.object(MODULE, "run", side_effect=[SimpleNamespace(stdout=identity), SimpleNamespace(stdout=plistlib.dumps(self.profile))]):
                report = MODULE.preflight(Path(directory) / "fake.provisionprofile", path)
            self.assertEqual(report["keychain_groups"], [f"{TEAM_ID}.{MODULE.BUNDLE_ID}"])
            self.assertTrue(report["checks"]["team_matches"])
            self.assertTrue(report["checks"]["keychain_groups_authorized"])

    def test_expired_or_future_profile_is_rejected(self):
        self.profile["ExpirationDate"] = self.now - dt.timedelta(seconds=1)
        self.assertFalse(self.report()["ready_for_signing_review"])
        self.profile["ExpirationDate"] = self.now + dt.timedelta(days=1)
        self.profile["CreationDate"] = self.now + dt.timedelta(seconds=1)
        self.assertFalse(self.report()["ready_for_signing_review"])

    def test_same_team_wrong_certificate_is_rejected(self):
        self.identities[0]["sha1"] = "0" * 40
        self.assertFalse(self.report()["checks"]["matching_private_key_identity_available"])

    def test_ios_profile_wrong_app_or_unauthorized_group_is_rejected(self):
        self.profile["Platform"] = ["iOS"]
        self.assertFalse(self.report()["ready_for_signing_review"])
        self.profile["Platform"] = ["OSX"]
        self.profile["Entitlements"]["com.apple.application-identifier"] = f"{TEAM_ID}.another-app"
        self.assertFalse(self.report()["checks"]["bundle_identifier_matches"])
        self.entitlements["keychain-access-groups"] = ["OTHERTEAM.com.example"]
        self.assertFalse(self.report()["checks"]["keychain_groups_authorized"])

    def test_profile_wildcard_does_not_authorize_a_literal_wildcard_in_app_groups(self):
        self.entitlements["keychain-access-groups"] = [f"{TEAM_ID}.*"]
        self.assertFalse(self.report()["checks"]["keychain_groups_concrete"])

    def test_missing_app_keychain_group_fails_preflight(self):
        self.entitlements["keychain-access-groups"] = []
        self.assertFalse(self.report()["ready_for_signing_review"])

    def test_identity_output_omits_personal_common_name(self):
        result = MODULE.parse_identities('1) ' + 'A' * 40 + ' "Apple Distribution: Test Personal Name (TEAM123456)"')
        self.assertEqual(result, [{"sha1": "A" * 40, "kind": "Apple Distribution", "team": TEAM_ID}])

    def test_keychain_cleanup_failure_is_not_reported_as_clean(self):
        self.assertFalse(MODULE.keychain_clean({"classic": {"add_status": 0, "delete_status": -25308}}))
        self.assertTrue(MODULE.keychain_clean({"classic": {"add_status": 0, "delete_status": 0, "after_delete_status": -25300}, "data_protection": {"add_status": -34018}}))


if __name__ == "__main__":
    unittest.main()
