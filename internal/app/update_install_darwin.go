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

func scheduleUpdateInstall(dmgPath, targetApp, expectedVersion, expectedBundleVersion string, parentPID int) error {
	if !filepath.IsAbs(dmgPath) || !filepath.IsAbs(targetApp) || parentPID <= 0 {
		return fmt.Errorf("invalid update paths or process id")
	}
	updateDir := filepath.Dir(dmgPath)
	logPath := filepath.Join(updateDir, "install-update.log")
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
expected=%s
expected_bundle=%s
parent=%d
log=%s
mount=""
backup=""
relaunch=0
exec >>"$log" 2>&1
echo "update installer started"
cleanup() {
  status=$?
  trap - EXIT
  if [ -n "$backup" ] && [ -d "$backup" ]; then
    rm -rf "$target"
    mv "$backup" "$target"
    echo "restored previous app after update failure"
  fi
  [ -z "$mount" ] || hdiutil detach "$mount" -quiet || true
  if [ "$relaunch" -eq 1 ] && [ -d "$target" ]; then
    /usr/bin/open -na "$target" || true
  fi
  rm -f "$dmg" "$0"
  exit "$status"
}
trap cleanup EXIT
parent_running() {
  kill -0 "$parent" 2>/dev/null || return 1
  state=$(ps -p "$parent" -o stat= 2>/dev/null | tr -d '[:space:]')
  case "$state" in Z*) return 1;; esac
  return 0
}
if parent_running; then
  parent_command=$(ps -p "$parent" -o command= 2>/dev/null || true)
  case "$parent_command" in
    *IntegTERM*) ;;
    *) echo "refusing to signal unexpected parent process"; exit 1;;
  esac
fi
attempt=0
while parent_running && [ "$attempt" -lt 150 ]; do
  attempt=$((attempt + 1))
  sleep 0.2
done
if parent_running; then
  echo "parent did not exit within 30 seconds; requesting termination"
  kill -TERM "$parent" 2>/dev/null || true
  attempt=0
  while parent_running && [ "$attempt" -lt 50 ]; do
    attempt=$((attempt + 1))
    sleep 0.2
  done
fi
if parent_running; then
  echo "parent did not respond to termination; forcing termination"
  kill -KILL "$parent" 2>/dev/null || true
  attempt=0
  while parent_running && [ "$attempt" -lt 25 ]; do
    attempt=$((attempt + 1))
    sleep 0.2
  done
fi
if parent_running; then
  echo "parent is still running; aborting without replacing the app"
  exit 1
fi
relaunch=1
mount=$(hdiutil attach "$dmg" -nobrowse -readonly -plist | awk -F'[<>]' '/<key>mount-point<\/key>/{getline; print $3; exit}')
[ -n "$mount" ]
source=$(find "$mount" -type d -name 'IntegTERM.app' -print -quit)
[ -n "$source" ]
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$source/Contents/Info.plist" | grep -Fx "$expected"
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$source/Contents/Info.plist" | grep -Fx "$expected_bundle"
/usr/bin/codesign --verify --deep --strict "$source"
backup="${target}.update-backup-$$"
mv "$target" "$backup"
/usr/bin/ditto "$source" "$target"
/usr/bin/codesign --verify --deep --strict "$target"
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$target/Contents/Info.plist" | grep -Fx "$expected"
/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$target/Contents/Info.plist" | grep -Fx "$expected_bundle"
rm -rf "$backup"
backup=""
echo "update installer completed"
`, quote(dmgPath), quote(targetApp), quote(expectedVersion), quote(expectedBundleVersion), parentPID, quote(logPath))
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
	command := exec.Command("/usr/bin/nohup", "/bin/sh", scriptPath)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	return command.Start()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
