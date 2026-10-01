//go:build darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellQuotePreservesLiteralValue(t *testing.T) {
	value := "/tmp/IntegTERM user's update.dmg"
	if got, want := shellQuote(value), `'/tmp/IntegTERM user'\''s update.dmg'`; got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestScheduleUpdateInstallRejectsRelativePaths(t *testing.T) {
	if err := scheduleUpdateInstall("update.dmg", "/Applications/IntegTERM.app", "1.26.0912", "1.26.09120001", os.Getpid()); err == nil {
		t.Fatal("scheduleUpdateInstall accepted a relative DMG path")
	}
	if err := scheduleUpdateInstall(filepath.Join(t.TempDir(), "update.dmg"), "IntegTERM.app", "1.26.0912", "1.26.09120001", os.Getpid()); err == nil {
		t.Fatal("scheduleUpdateInstall accepted a relative app path")
	}
}

func TestInstallerKeepsValidatedAppWhenBackupCleanupFails(t *testing.T) {
	runInstallerFixture(t, "cleanup")
}
func TestInstallerRestoresPreviousAppWhenCopyFails(t *testing.T) {
	runInstallerFixture(t, "copy")
}
func runInstallerFixture(t *testing.T, failure string) {
	t.Helper()
	root := t.TempDir()
	commands := filepath.Join(root, "bin")
	mount := filepath.Join(root, "mount")
	target := filepath.Join(root, "IntegTERM.app")
	source := filepath.Join(mount, "IntegTERM.app")
	for _, dir := range []string{commands, target, source} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{filepath.Join(target, "old"), filepath.Join(source, "new")} {
		if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stubs := map[string]string{
		"hdiutil":    "if [ \"$1\" = attach ]; then printf '<key>mount-point</key>\\n<string>%s</string>\\n' \"$FIXTURE_MOUNT\"; fi",
		"PlistBuddy": "case \"$2\" in *Short*) echo 1.0.0;; *) echo 1.0.00001;; esac",
		"codesign":   "exit 0",
		"open":       "exit 0",
		"ditto":      "/bin/cp -R \"$1\" \"$2\"; if [ \"$FIXTURE_FAILURE\" = copy ]; then exit 1; fi",
		"rm":         "case \"$*\" in *update-backup-*) if [ \"$FIXTURE_FAILURE\" = cleanup ]; then /bin/rm -f \"$2/old\"; exit 1; fi;; esac; exec /bin/rm \"$@\"",
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(commands, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	script := updateInstallScript(filepath.Join(root, "update.dmg"), target, "1.0.0", "1.0.00001", 123, filepath.Join(root, "install.log"))
	// 僅替換 OS 介接指令；檔案搬移與失敗回復執行真正的 shell 邏輯。
	script = strings.NewReplacer("/usr/libexec/PlistBuddy", filepath.Join(commands, "PlistBuddy"), "/usr/bin/codesign", filepath.Join(commands, "codesign"), "/usr/bin/ditto", filepath.Join(commands, "ditto"), "/usr/bin/open", filepath.Join(commands, "open")).Replace(script)
	script = strings.Replace(script, "set -eu", "set -eu\nkill() { return 1; }", 1)
	scriptPath := filepath.Join(root, "install.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", scriptPath)
	cmd.Env = append(os.Environ(), "PATH="+commands+":/usr/bin:/bin", "FIXTURE_MOUNT="+mount, "FIXTURE_FAILURE="+failure)
	err := cmd.Run()
	expected := "new"
	if failure == "copy" {
		expected = "old"
		if err == nil {
			t.Fatal("failed copy reported success")
		}
	}
	if _, statErr := os.Stat(filepath.Join(target, expected)); statErr != nil {
		log, _ := os.ReadFile(filepath.Join(root, "install.log"))
		t.Fatalf("lost %s app: %v; installer=%v; log=%s", expected, statErr, err, log)
	}
}
