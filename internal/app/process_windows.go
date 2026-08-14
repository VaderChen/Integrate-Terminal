//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const windowsStillActive = 259

func configureBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func configureDetachedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func backgroundProcessRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == windowsStillActive
}

func executableCommand(targetPath string, _ os.FileInfo) (*exec.Cmd, error) {
	switch strings.ToLower(filepath.Ext(targetPath)) {
	case ".exe", ".com":
		return exec.Command(targetPath), nil
	case ".bat", ".cmd":
		commandInterpreter := strings.TrimSpace(os.Getenv("COMSPEC"))
		if commandInterpreter == "" {
			commandInterpreter = "cmd.exe"
		}
		return exec.Command(commandInterpreter, "/d", "/s", "/c", targetPath), nil
	default:
		return nil, fmt.Errorf("file is not executable")
	}
}
