//go:build !windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func configureDetachedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func backgroundProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func executableCommand(targetPath string, info os.FileInfo) (*exec.Cmd, error) {
	if info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("file is not executable")
	}
	return exec.Command(targetPath), nil
}
