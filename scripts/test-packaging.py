#!/usr/bin/env python3
"""Exercise packaging failure paths with fake apps and signing tools."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

PROJECT = Path(__file__).resolve().parents[1]


class PackagingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="integterm-packaging-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "tools"
        self.bin.mkdir()
        self.env = dict(os.environ, PATH=f"{self.bin}:{os.environ['PATH']}")
        for key in ("SIGNING_CONFIG", "TEAM_ID", "APP_CERT", "INSTALLER_CERT", "BUNDLE_ID", "PROVISION_PROFILE_PATH", "SIGNING_KEYCHAIN", "CERT_DIR", "CERT_P12_PASSWORD", "TEMP_KEYCHAIN_PASSWORD"):
            self.env.pop(key, None)

    def tool(self, name, body):
        target = self.bin / name
        target.write_text(f"#!{sys.executable}\n" + body)
        target.chmod(0o755)

    def script(self, name):
        target = self.root / name
        shutil.copyfile(PROJECT / name, target)
        return target

    def run_script(self, script):
        return subprocess.run(["/bin/zsh", str(script)], cwd=self.root,
                              env=self.env, capture_output=True, text=True, timeout=15)

    def setup_dmg(self):
        script = self.script("package-dmg.sh")
        build = self.root / "build.sh"
        build.write_text(f"#!{sys.executable}\n" + '''
from pathlib import Path
import plistlib
root = Path(__file__).parent
(root / "build-called").touch()
contents = root / "build/bin/IntegTERM.app/Contents"
contents.mkdir(parents=True)
(contents / "Info.plist").write_bytes(plistlib.dumps({"CFBundleShortVersionString": "1.2.3"}))
''')
        build.chmod(0o755)
        self.tool("hdiutil", '''
import os, sys
from pathlib import Path
args = sys.argv[1:]
root = Path(args[args.index("-srcfolder") + 1])
assert (root / "IntegTERM.app/Contents/Info.plist").is_file()
assert (root / "Applications").is_symlink()
if os.environ.get("TEST_HDI_FAIL"):
    sys.exit(1)
Path(args[-1]).write_text("new image")
''')
        return script

    def test_dmg_builds_missing_app_before_reading_version(self):
        result = self.run_script(self.setup_dmg())
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue((self.root / "build-called").exists())
        self.assertEqual((self.root / "dist/IntegTERM-1.2.3.dmg").read_text(), "new image")
        self.assertEqual(list((self.root / "dist").glob(".dmg-staging.*")), [])

    def test_failed_dmg_keeps_previous_image(self):
        script = self.setup_dmg()
        dist = self.root / "dist"
        dist.mkdir()
        old = dist / "IntegTERM-1.2.3.dmg"
        old.write_text("previous image")
        self.env["TEST_HDI_FAIL"] = "1"
        self.assertNotEqual(self.run_script(script).returncode, 0)
        self.assertEqual(old.read_text(), "previous image")
        self.assertEqual(list(dist.glob(".dmg-staging.*")), [])

    def setup_appstore(self, import_assets=True):
        import datetime as dt
        import plistlib
        script = self.script("package-appstore.sh")
        (self.root / "scripts").mkdir()
        shutil.copyfile(PROJECT / "scripts/appstore_config.py", self.root / "scripts/appstore_config.py")
        contents = self.root / "build/bin/IntegTERM.app/Contents/MacOS"
        contents.mkdir(parents=True)
        (contents / "IntegTERM").write_text("fake executable")
        (contents.parent / "Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": "com.vader.integterm"}))
        (self.root / "cert").mkdir()
        if import_assets:
            (self.root / "cert/distribution.key").write_text("fake asset")
        now = dt.datetime.now()
        profile = {"TeamIdentifier": ["TEAM123456"], "ApplicationIdentifierPrefix": ["TEAM123456"],
                   "Platform": ["OSX"], "CreationDate": now - dt.timedelta(days=1), "ExpirationDate": now + dt.timedelta(days=1),
                   "DeveloperCertificates": [b"fake-public-certificate"],
                   "Entitlements": {"com.apple.application-identifier": "TEAM123456.com.vader.integterm", "keychain-access-groups": ["TEAM123456.*"]}}
        (self.root / "cert/IntegTerm.provisionprofile").write_bytes(plistlib.dumps(profile))
        shutil.copyfile(PROJECT / "entitlements.plist", self.root / "entitlements.plist")
        self.env["TEST_SECURITY_LOG"] = str(self.root / "security.jsonl")
        self.env["TEST_ENTITLEMENTS"] = str(self.root / "signed-entitlements.plist")
        self.tool("security", '''
import hashlib, json, os, sys
from pathlib import Path
args = sys.argv[1:]
with open(os.environ["TEST_SECURITY_LOG"], "a") as log:
    log.write(json.dumps(args) + "\\n")
if args[0] in ("list-keychains", "default-keychain"):
    raise AssertionError("global Keychain mutation is forbidden")
if args[0] == "cms":
    sys.stdout.buffer.write(Path(args[-1]).read_bytes())
elif args[0] == "create-keychain":
    Path(args[-1]).touch()
elif args[0] == "delete-keychain":
    Path(args[-1]).unlink()
elif args[0] == "find-identity":
    print('1) ' + hashlib.sha1(b"fake-public-certificate").hexdigest().upper() + ' "Apple Distribution: Example Person (TEAM123456)"')
    print('2) ' + 'A' * 40 + ' "Mac Installer Distribution: Example Person (TEAM123456)"')
''')
        self.tool("ditto", '''
import os, shutil, sys
if os.environ.get("TEST_DITTO_FAIL"):
    sys.exit(1)
shutil.copytree(sys.argv[1], sys.argv[2])
''')
        self.tool("codesign", '''
import os, shutil, sys
if "--entitlements" in sys.argv:
    shutil.copyfile(sys.argv[sys.argv.index("--entitlements") + 1], os.environ["TEST_ENTITLEMENTS"])
''')
        self.tool("productbuild", '''
import sys
from pathlib import Path
Path(sys.argv[-1]).write_text("fake signed package")
''')
        for name in ("pkgutil", "xattr"):
            self.tool(name, "pass\n")
        return script

    def test_signing_cleanup_runs_when_staging_fails(self):
        script = self.setup_appstore()
        self.env["TEST_DITTO_FAIL"] = "1"
        result = self.run_script(script)
        self.assertNotEqual(result.returncode, 0)
        calls = [json.loads(line) for line in (self.root / "security.jsonl").read_text().splitlines()]
        self.assertFalse(any(call[0] in ("default-keychain", "list-keychains") for call in calls))
        deleted = [call[-1] for call in calls if call[0] == "delete-keychain"]
        self.assertEqual(len(deleted), 1, result.stdout + result.stderr)
        self.assertFalse(Path(deleted[0]).parent.exists())

    def test_profile_drives_concrete_entitlements_and_isolated_signing(self):
        import plistlib
        result = self.run_script(self.setup_appstore())
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        entitlements = plistlib.loads((self.root / "signed-entitlements.plist").read_bytes())
        self.assertEqual(entitlements["com.apple.application-identifier"], "TEAM123456.com.vader.integterm")
        self.assertEqual(entitlements["keychain-access-groups"], ["TEAM123456.com.vader.integterm"])
        self.assertTrue(entitlements["com.apple.security.network.server"])
        self.assertTrue(entitlements["com.apple.security.app-sandbox"])
        calls = [json.loads(line) for line in (self.root / "security.jsonl").read_text().splitlines()]
        keychain = next(call[-1] for call in calls if call[0] == "create-keychain")
        for call in calls:
            if call[0] in ("find-identity", "import", "set-key-partition-list"):
                self.assertIn(keychain, call)
            self.assertNotIn(call[0], ("default-keychain", "list-keychains"))
        self.assertFalse(Path(keychain).exists())
        self.assertNotIn("Example Person", result.stdout + result.stderr)

    def test_environment_overrides_local_signing_config(self):
        script = self.setup_appstore(import_assets=False)
        (self.root / "cert/signing.json").write_text(json.dumps({"TEAM_ID": "WRONG12345", "APP_CERT": "invalid", "INSTALLER_CERT": "invalid"}))
        self.env.update(TEAM_ID="TEAM123456", APP_CERT="Apple Distribution: Example Person (TEAM123456)", INSTALLER_CERT="Mac Installer Distribution: Example Person (TEAM123456)")
        result = self.run_script(script)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_wrong_configured_team_rejects_before_import(self):
        script = self.setup_appstore()
        self.env["TEAM_ID"] = "WRONG12345"
        result = self.run_script(script)
        self.assertNotEqual(result.returncode, 0)
        calls = [json.loads(line) for line in (self.root / "security.jsonl").read_text().splitlines()]
        self.assertFalse(any(call[0] in ("create-keychain", "import", "find-identity") for call in calls))


class AppStoreConfigTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        import importlib.util
        spec = importlib.util.spec_from_file_location("appstore_config", PROJECT / "scripts/appstore_config.py")
        cls.helper = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.helper)

    def test_profile_prefix_can_differ_from_team_but_group_stays_concrete(self):
        profile = {"TeamIdentifier": ["TEAM123456"], "ApplicationIdentifierPrefix": ["PREFIX1234"],
                   "Entitlements": {"com.apple.application-identifier": "PREFIX1234.com.example.app", "keychain-access-groups": ["PREFIX1234.*"]}}
        result = self.helper.resolve_entitlements(profile, {}, "com.example.app")
        self.assertEqual(result["com.apple.developer.team-identifier"], "TEAM123456")
        self.assertEqual(result["keychain-access-groups"], ["PREFIX1234.com.example.app"])
        profile["Entitlements"]["keychain-access-groups"] = ["ANOTHER123.*"]
        with self.assertRaises(ValueError):
            self.helper.resolve_entitlements(profile, {}, "com.example.app")

    def test_same_team_certificate_must_be_authorized_by_profile(self):
        import hashlib
        good = hashlib.sha1(b"authorized").hexdigest().upper()
        output = f'1) {good} "Apple Distribution: Example (TEAM123456)"'
        with self.assertRaises(ValueError):
            self.helper.select_identity(output, "app", "TEAM123456", profile={"DeveloperCertificates": [b"different"]})
        self.assertEqual(self.helper.select_identity(output, "app", "TEAM123456", profile={"DeveloperCertificates": [b"authorized"]}), good)

    def test_ambiguous_installer_requires_explicit_selection(self):
        output = '1) ' + 'A' * 40 + ' "Mac Installer Distribution: Example (TEAM123456)"\n'
        output += '2) ' + 'B' * 40 + ' "Mac Installer Distribution: Example (TEAM123456)"'
        with self.assertRaises(ValueError):
            self.helper.select_identity(output, "installer", "TEAM123456")
        self.assertEqual(self.helper.select_identity(output, "installer", "TEAM123456", "B" * 40), "B" * 40)



class BuildPrivacyTests(unittest.TestCase):
    """Run real Go/CGO and Swift compilers through isolated build workflows."""
    setUp = PackagingTests.setUp
    tool = PackagingTests.tool

    def setup_workflow(self, name):
        script = self.root / name
        # Keep personal shell startup files outside the test. All build script
        # logic, exported flags and compiler entry points remain unchanged.
        source = (PROJECT / name).read_text()
        source = source.replace('source "$HOME/.zshrc"', ': # isolated shell configuration')
        script.write_text(source)
        (self.root / "go.mod").write_text((PROJECT / "go.mod").read_text())
        (self.root / "frontend/node_modules").mkdir(parents=True)
        (self.root / "frontend/wailsjs").mkdir()
        (self.root / "internal/version").mkdir(parents=True)
        (self.root / "wails.json").write_text('{"info":{}}')
        bridge = self.root / "scripts/build-storekit2-bridge.sh"
        bridge.parent.mkdir()
        bridge.write_text("#!/bin/sh\nexit 0\n")
        bridge.chmod(0o755)
        icon = self.root / "sync-app-icon.sh"
        icon.write_text("#!/bin/sh\nexit 0\n")
        icon.chmod(0o755)
        real_go = shutil.which("go")
        self.assertIsNotNone(real_go)
        # A source checkout path containing spaces exercises the same argument
        # handling as a project on an external volume.
        probe = self.root / "private build root"
        probe.mkdir()
        (probe / "go.mod").write_text("module example.com/build-privacy-probe\n\ngo 1.26.8\n")
        (probe / "main.go").write_text('''package main
/*
#include <stdio.h>
const char *fixture_source_path(void);
*/
import "C"
import ("fmt"; "runtime")
func main() {
    _, file, _, _ := runtime.Caller(0)
    fmt.Println(file)
    fmt.Println(C.GoString(C.fixture_source_path()))
}
''')
        (probe / "fixture.c").write_text('const char *fixture_source_path(void) { return __FILE__; }\n')
        self.env["TEST_REAL_GO"] = real_go
        self.env["TEST_BUILD_PROBE"] = str(probe)
        self.env["TEST_BUILD_OUTPUT"] = str(self.root / "probe-binary")
        self.env["TEST_WAILS_RECORD"] = str(self.root / "wails-record.json")
        # A caller's earlier opt-out must not silently disable the project's
        # privacy setting. The unrelated buildvcs option must keep working.
        self.env["GOFLAGS"] = "-buildvcs=false -trimpath=false"
        self.tool("go", "raise SystemExit(0)\n")
        self.tool("node", "raise SystemExit(0)\n")
        self.tool("npm", '''
from pathlib import Path
Path("dist").mkdir(exist_ok=True)
''')
        self.tool("codesign", "raise AssertionError('fixture must stop before signing')\n")
        self.tool("wails", '''
import json, os, subprocess, sys
from pathlib import Path
Path(os.environ["TEST_WAILS_RECORD"]).write_text(json.dumps(sys.argv[1:]))
# Wails ultimately invokes Go build; run that compiler for a small fixture with
# the environment inherited from the actual production/development script.
subprocess.run([os.environ["TEST_REAL_GO"], "build", "-o", os.environ["TEST_BUILD_OUTPUT"], "."],
               cwd=os.environ["TEST_BUILD_PROBE"], check=True)
raise SystemExit(73)  # Stop before launch/signing, after verifying compilation.
''')
        return script, probe

    def test_production_and_dev_binaries_omit_go_and_c_source_roots(self):
        for name in ("build.sh", "run.sh"):
            with self.subTest(script=name):
                # Each workflow gets a fresh directory and tool set.
                original_root, original_bin, original_env = self.root, self.bin, self.env
                try:
                    self.root = original_root / name.removesuffix(".sh")
                    self.root.mkdir()
                    self.bin = self.root / "tools"
                    self.bin.mkdir()
                    self.env = dict(original_env, PATH=f"{self.bin}:{os.environ['PATH']}")
                    script, probe = self.setup_workflow(name)
                    result = subprocess.run(["/bin/zsh", str(script)], cwd=self.root,
                                            env=self.env, capture_output=True, text=True, timeout=120)
                    self.assertEqual(result.returncode, 73, result.stdout + result.stderr)
                    binary = self.root / "probe-binary"
                    output = subprocess.check_output([str(binary)], text=True)
                    self.assertIn("build-privacy-probe/main.go", output)
                    self.assertEqual(Path(output.splitlines()[1]).name, "fixture.c")
                    self.assertNotIn(str(probe), output)
                    self.assertNotIn(str(probe).encode(), binary.read_bytes())
                    command = json.loads((self.root / "wails-record.json").read_text())
                    self.assertEqual(command[0], "build" if name == "build.sh" else "dev")
                    if name == "build.sh":
                        self.assertIn("-trimpath", command)
                finally:
                    self.root, self.bin, self.env = original_root, original_bin, original_env

    def test_swift_bridge_source_path_and_library_id_are_portable(self):
        import ctypes
        project = self.root / "private Swift project"
        script = project / "scripts/build-storekit2-bridge.sh"
        script.parent.mkdir(parents=True)
        shutil.copyfile(PROJECT / "scripts/build-storekit2-bridge.sh", script)
        source = project / "internal/purchase/swift/StoreKit2Bridge.swift"
        source.parent.mkdir(parents=True)
        # No StoreKit or purchase APIs are invoked; the real bridge build script
        # compiles a harmless function which deliberately exposes #filePath.
        source.write_text('''import Darwin
@_cdecl("integterm_fixture_source_path")
public func fixtureSourcePath() -> UnsafeMutablePointer<CChar>? {
    return strdup(#filePath)
}
''')
        result = subprocess.run(["/bin/zsh", str(script)], cwd=self.root,
                                env=self.env, capture_output=True, text=True, timeout=120)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        library = project / "internal/purchase/native/libintegtermstorekit2.dylib"
        loaded = ctypes.CDLL(str(library))
        loaded.integterm_fixture_source_path.restype = ctypes.c_void_p
        pointer = loaded.integterm_fixture_source_path()
        self.assertIsNotNone(pointer)
        try:
            path = ctypes.string_at(pointer).decode()
        finally:
            libc = ctypes.CDLL(None)
            libc.free.argtypes = [ctypes.c_void_p]
            libc.free(pointer)
        self.assertTrue(path.endswith("internal/purchase/swift/StoreKit2Bridge.swift"), path)
        self.assertNotIn(str(project), path)
        self.assertNotIn(str(project).encode(), library.read_bytes())
        ids = subprocess.check_output(["otool", "-D", str(library)], text=True).splitlines()
        self.assertEqual(ids[1].strip(), "@rpath/libintegtermstorekit2.dylib")
        load_commands = subprocess.check_output(["otool", "-l", str(library)], text=True)
        self.assertRegex(load_commands, r"minos\s+12\.0")

if __name__ == "__main__":
    unittest.main()
