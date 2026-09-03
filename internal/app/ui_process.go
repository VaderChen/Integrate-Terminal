package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const foregroundUIPIDFile = "foreground-ui.pids"

var foregroundUIPIDMu sync.Mutex

func (a *App) foregroundUIPIDPath() string {
	return filepath.Join(a.store.BaseDir(), foregroundUIPIDFile)
}

// RegisterForegroundUI records a UI process so the tray service can close it
// even when macOS launches the app through open and does not preserve the
// executable path in the process command line.
func (a *App) RegisterForegroundUI(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid foreground UI pid: %d", pid)
	}
	if err := a.store.Ensure(); err != nil {
		return err
	}

	foregroundUIPIDMu.Lock()
	defer foregroundUIPIDMu.Unlock()

	pids := readForegroundUIPIDs(a.foregroundUIPIDPath())
	pids = appendPIDIfMissing(pids, pid)
	return writeForegroundUIPIDs(a.foregroundUIPIDPath(), pids)
}

// UnregisterForegroundUI removes a UI process after Wails has completed its
// shutdown. Stale entries are discarded whenever the file is read.
func (a *App) UnregisterForegroundUI(pid int) error {
	if pid <= 0 {
		return nil
	}

	foregroundUIPIDMu.Lock()
	defer foregroundUIPIDMu.Unlock()

	pids := readForegroundUIPIDs(a.foregroundUIPIDPath())
	remaining := make([]int, 0, len(pids))
	for _, candidate := range pids {
		if candidate != pid {
			remaining = append(remaining, candidate)
		}
	}
	if len(remaining) == 0 {
		if err := os.Remove(a.foregroundUIPIDPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeForegroundUIPIDs(a.foregroundUIPIDPath(), remaining)
}

// ForegroundUIProcessIDs returns live UI PIDs and cleans up stale entries.
func (a *App) ForegroundUIProcessIDs() []int {
	foregroundUIPIDMu.Lock()
	defer foregroundUIPIDMu.Unlock()

	path := a.foregroundUIPIDPath()
	pids := readForegroundUIPIDs(path)
	live := make([]int, 0, len(pids))
	for _, pid := range pids {
		if pid > 0 && backgroundProcessRunning(pid) {
			live = appendPIDIfMissing(live, pid)
		}
	}
	if len(live) == 0 {
		_ = os.Remove(path)
		return nil
	}
	_ = writeForegroundUIPIDs(path, live)
	return append([]int(nil), live...)
}

func readForegroundUIPIDs(path string) []int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pids := make([]int, 0)
	for _, line := range strings.Split(string(data), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && pid > 0 {
			pids = appendPIDIfMissing(pids, pid)
		}
	}
	return pids
}

func appendPIDIfMissing(pids []int, pid int) []int {
	for _, candidate := range pids {
		if candidate == pid {
			return pids
		}
	}
	return append(pids, pid)
}

func writeForegroundUIPIDs(path string, pids []int) error {
	if len(pids) == 0 {
		return nil
	}
	data := make([]byte, 0, len(pids)*12)
	for _, pid := range pids {
		data = append(data, strconv.Itoa(pid)...)
		data = append(data, '\n')
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".foreground-ui-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
