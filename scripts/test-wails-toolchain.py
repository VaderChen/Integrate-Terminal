#!/usr/bin/env python3
"""驗證專案 CLI 選用，以及真正的 Wails / Go embed 編譯流程。"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

PROJECT = Path(__file__).resolve().parents[1]


class WailsToolchainTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix="integterm-wails-test-")
        cls.addClassCleanup(cls.temp.cleanup)
        cls.root = Path(cls.temp.name)
        cls.go_version = "go" + next(line.split()[1] for line in (PROJECT / "go.mod").read_text().splitlines() if line.startswith("go "))
        cls.env = dict(os.environ, GOTOOLCHAIN=cls.go_version, INTEGTERM_TOOL_CACHE=str(cls.root / "tool cache"))
        cls.env.pop("GOROOT", None)
        cls.wails_version = subprocess.check_output(["go", "list", "-m", "-f", "{{.Version}}", "github.com/wailsapp/wails/v2"], cwd=PROJECT, env=cls.env, text=True).strip()
        cls.tools_version = subprocess.check_output(["go", "list", "-m", "-f", "{{.Version}}", "golang.org/x/tools"], cwd=PROJECT, env=cls.env, text=True).strip()
        stale = cls.root / "old tools"
        stale.mkdir()
        old_cli = stale / "wails"
        old_cli.write_text('#!/bin/sh\necho "錯誤：執行了全域舊 CLI" >&2\nexit 91\n')
        old_cli.chmod(0o755)
        cls.env["PATH"] = str(stale) + os.pathsep + cls.env["PATH"]
        cls.original_modules = {name: (PROJECT / name).read_bytes() for name in ("go.mod", "go.sum")}
        result = subprocess.run(["zsh", str(PROJECT / "scripts/ensure-wails.sh")], cwd=PROJECT, env=cls.env, capture_output=True, text=True, timeout=180)
        if result.returncode:
            raise AssertionError(result.stdout + result.stderr)
        cls.cli = Path(result.stdout.strip())

    def test_cli_matches_project_and_reuses_verified_cache(self):
        metadata = subprocess.check_output(["go", "version", "-m", str(self.cli)], env=self.env, text=True)
        self.assertEqual(metadata.splitlines()[0].split()[-1], self.go_version)
        records = [line.split() for line in metadata.splitlines()[1:]]
        self.assertTrue(any(row[:3] == ["mod", "github.com/wailsapp/wails/v2", self.wails_version] for row in records))
        self.assertTrue(any(row[:3] == ["dep", "golang.org/x/tools", self.tools_version] for row in records))
        modified = self.cli.stat().st_mtime_ns
        result = subprocess.run(["zsh", str(PROJECT / "scripts/ensure-wails.sh")], cwd=PROJECT, env=self.env, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), str(self.cli))
        self.assertEqual(self.cli.stat().st_mtime_ns, modified)
        for name, original in self.original_modules.items():
            self.assertEqual((PROJECT / name).read_bytes(), original, name)

    def test_real_wails_can_analyze_embed_and_build_with_project_go(self):
        fixture = self.root / "embed project"
        fixture.mkdir()
        (fixture / "go.mod").write_text(f"module example.com/wails-toolchain-smoke\n\ngo {self.go_version[2:]}\n\nrequire github.com/wailsapp/wails/v2 {self.wails_version}\n")
        shutil.copy2(PROJECT / "go.sum", fixture / "go.sum")
        (fixture / "wails.json").write_text(json.dumps({"name": "toolchain-smoke", "outputfilename": "toolchain-smoke", "frontend:dir": "frontend"}))
        (fixture / "frontend").mkdir()
        (fixture / "payload.txt").write_text("embed-smoke")
        (fixture / "main.go").write_text('''package main
import ("embed"; "fmt"; "runtime")
//go:embed payload.txt
var files embed.FS
func main() {
    data, err := files.ReadFile("payload.txt")
    if err != nil { panic(err) }
    fmt.Println(string(data), runtime.Version())
}
''')
        result = subprocess.run([str(self.cli), "build", "-s", "-skipbindings", "-nopackage", "-m"], cwd=fixture, env=self.env, capture_output=True, text=True, timeout=120)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        binary = fixture / "build/bin/toolchain-smoke"
        output = subprocess.check_output([str(binary)], text=True)
        self.assertEqual(output.strip(), "embed-smoke " + self.go_version)


if __name__ == "__main__":
    unittest.main()
