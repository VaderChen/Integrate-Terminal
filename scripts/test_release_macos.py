"""正式版公證流程的失敗保護測試，不接觸真實簽章或 Apple 認證。"""
import importlib.util
import os
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release_macos', Path(__file__).with_name('release-macos.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def test_reject_adhoc_or_missing_profile(self):
        for env in [{}, {'INTEGTERM_CODESIGN_IDENTITY': '-'},
                    {'INTEGTERM_CODESIGN_IDENTITY': 'Developer ID Application: Test'}]:
            with self.subTest(env=env), patch.dict(os.environ, env, clear=True):
                with self.assertRaises(ValueError):
                    release.configuration()

    def test_rejected_notarization_cannot_continue(self):
        with patch.object(release, 'run', return_value='{"id":"test-only","status":"Invalid"}'):
            with self.assertRaises(RuntimeError):
                release.notarize(Path('test.dmg'), 'test-profile')

    def test_accepted_notarization_returns_receipt(self):
        with patch.object(release, 'run', return_value='{"id":"test-only","status":"Accepted"}'):
            self.assertEqual(release.notarize(Path('test.dmg'), 'test-profile')['status'], 'Accepted')

    def test_hardened_runtime_and_stable_identifier(self):
        with patch.object(release, 'run') as run:
            release.sign(Path('IntegTERM.app'), 'Developer ID Application: Test', 'com.vader.integterm')
            args = run.call_args.args[0]
            self.assertIn('runtime', args)
            self.assertIn('--timestamp', args)
            self.assertEqual(args[args.index('--identifier') + 1], 'com.vader.integterm')


if __name__ == '__main__':
    unittest.main()
