package app

import (
	"fmt"
	"path"
	"strings"
)

func (a *App) StatRemoteEntry(tabID string, targetPath string) (map[string]any, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil, fmt.Errorf("path is required")
	}

	parentPath := path.Dir(targetPath)
	targetName := path.Base(targetPath)
	entries := a.ListRemote(tabID, parentPath)
	for _, entry := range entries {
		if entry.Name == targetName {
			return map[string]any{
				"name":     entry.Name,
				"path":     entry.Path,
				"size":     entry.Size,
				"modified": entry.Modified,
				"isDir":    entry.IsDir,
				"side":     entry.Side,
			}, nil
		}
	}
	return nil, fmt.Errorf("remote path not found: %s", targetPath)
}
