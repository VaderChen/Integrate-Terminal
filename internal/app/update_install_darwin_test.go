//go:build darwin

package app

import (
	"os"
	"path/filepath"
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
