//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func startDetachedCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

func scheduleUpdateInstall(dmgPath, targetApp, _ string, parentPID int) error {
	if !filepath.IsAbs(dmgPath) || !filepath.IsAbs(targetApp) || parentPID <= 0 {
		return fmt.Errorf("invalid update paths or process id")
	}
	updateDir := filepath.Dir(dmgPath)
	script, err := os.CreateTemp(updateDir, ".install-*.sh")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	quote := shellQuote
	contents := fmt.Sprintf(`#!/bin/sh
set -eu
dmg=%s
target=%s
parent=%d
mount=""
cleanup() { [ -z "$mount" ] || hdiutil detach "$mount" -quiet || true; rm -f "$dmg" "$0"; }
trap cleanup EXIT
while kill -0 "$parent" 2>/dev/null; do sleep 0.2; done
mount=$(hdiutil attach "$dmg" -nobrowse -readonly -plist | awk -F'[<>]' '/<key>mount-point<\/key>/{getline; print $3; exit}')
[ -n "$mount" ]
source=$(find "$mount" -type d -name 'IntegTERM.app' -print -quit)
[ -n "$source" ]
/usr/bin/codesign --verify --deep --strict "$source"
backup="${target}.update-backup-$$"
mv "$target" "$backup"
if ! /usr/bin/ditto "$source" "$target"; then mv "$backup" "$target"; exit 1; fi
if ! /usr/bin/codesign --verify --deep --strict "$target"; then rm -rf "$target"; mv "$backup" "$target"; exit 1; fi
rm -rf "$backup"
/usr/bin/open -a "$target"
`, quote(dmgPath), quote(targetApp), parentPID)
	if _, err := script.WriteString(contents); err != nil {
		_ = script.Close()
		return err
	}
	if err := script.Chmod(0o700); err != nil {
		_ = script.Close()
		return err
	}
	if err := script.Close(); err != nil {
		return err
	}
	return exec.Command("/bin/sh", scriptPath).Start()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
