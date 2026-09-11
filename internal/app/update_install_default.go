//go:build !darwin

package app

import (
	"fmt"
	"os/exec"
)

func scheduleUpdateInstall(_, _, _ string, _ int) error {
	return fmt.Errorf("automatic installation is only supported on macOS DMG updates")
}

func startDetachedCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}
