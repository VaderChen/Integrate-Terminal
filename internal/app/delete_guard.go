package app

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (a *App) validateDeleteTarget(tabID, side, target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("delete path is required")
	}
	a.stateMu.RLock()
	current := ""
	for _, tab := range a.tabs {
		if tab.ID == tabID {
			if side == "local" {
				current = tab.LocalPath
			} else {
				current = tab.RemotePath
			}
			break
		}
	}
	a.stateMu.RUnlock()
	switch side {
	case "local":
		if clean := filepath.Clean(target); clean == "." || clean == ".." {
			return fmt.Errorf("refusing to delete current/parent directory")
		}
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if filepath.Dir(targetAbs) == targetAbs {
			return fmt.Errorf("refusing to delete a filesystem root")
		}
		if current == "" {
			current, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		currentAbs, err := filepath.Abs(current)
		if err != nil {
			return err
		}
		// RemoveAll unlinks symlinks rather than following them. For directories,
		// resolving aliases makes the ancestor check cover symlinked parent paths.
		if info, err := os.Lstat(targetAbs); err == nil && info.Mode()&os.ModeSymlink == 0 {
			if resolved, err := filepath.EvalSymlinks(targetAbs); err == nil {
				targetAbs = resolved
			}
		}
		if resolved, err := filepath.EvalSymlinks(currentAbs); err == nil {
			currentAbs = resolved
		}
		if deleteTargetContains("local", targetAbs, currentAbs) {
			return fmt.Errorf("refusing to delete the current directory or an ancestor")
		}
	case "remote":
		clean := path.Clean(target)
		if clean == "/" || clean == "." || clean == ".." {
			return fmt.Errorf("refusing to delete the remote root/current/parent directory")
		}
		if current == "" {
			return fmt.Errorf("cannot determine the current remote directory")
		}
		if !path.IsAbs(clean) {
			clean = path.Join(current, clean)
		}
		if deleteTargetContains("remote", clean, path.Clean(current)) {
			return fmt.Errorf("refusing to delete the current remote directory or an ancestor")
		}
	default:
		return fmt.Errorf("unsupported side: %s", side)
	}
	return nil
}
