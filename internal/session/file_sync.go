package session

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

func (m *Manager) CompareDirectories(tabID string, localPath string, remotePath string) ([]model.FileComparison, error) {
	client, err := m.clientForTab(tabID)
	if err != nil {
		return nil, err
	}

	localRoot := filepath.Clean(strings.TrimSpace(localPath))
	if localRoot == "." || localRoot == "" {
		return nil, fmt.Errorf("local path is required")
	}
	localInfo, err := os.Stat(localRoot)
	if err != nil {
		return nil, err
	}
	if !localInfo.IsDir() {
		return nil, fmt.Errorf("local path is not a directory: %s", localRoot)
	}

	remoteRoot := normalizeRemoteSyncRoot(remotePath)
	if _, err := client.Stat(remoteRoot); err != nil {
		if entries, listErr := client.List(remoteRoot); listErr != nil {
			return nil, fmt.Errorf("remote path is not a directory: %s: %w", remoteRoot, err)
		} else if entries == nil {
			return nil, fmt.Errorf("remote path is not a directory: %s", remoteRoot)
		}
	}

	localEntries, err := collectLocalSyncEntries(localRoot)
	if err != nil {
		return nil, err
	}
	remoteEntries, err := collectRemoteSyncEntries(client, remoteRoot)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]struct{}, len(localEntries)+len(remoteEntries))
	for relativePath := range localEntries {
		paths[relativePath] = struct{}{}
	}
	for relativePath := range remoteEntries {
		paths[relativePath] = struct{}{}
	}

	sortedPaths := make([]string, 0, len(paths))
	for relativePath := range paths {
		sortedPaths = append(sortedPaths, relativePath)
	}
	sort.Strings(sortedPaths)

	comparisons := make([]model.FileComparison, 0, len(sortedPaths))
	for _, relativePath := range sortedPaths {
		localEntry, localExists := localEntries[relativePath]
		remoteEntry, remoteExists := remoteEntries[relativePath]
		comparison := model.FileComparison{
			RelativePath: relativePath,
			LocalExists:  localExists,
			RemoteExists: remoteExists,
		}
		if localExists {
			comparison.LocalSize = localEntry.Size
			comparison.LocalModified = localEntry.Modified
			comparison.LocalDirectory = localEntry.IsDir
		}
		if remoteExists {
			comparison.RemoteSize = remoteEntry.Size
			comparison.RemoteModified = remoteEntry.Modified
			comparison.RemoteDirectory = remoteEntry.IsDir
		}
		comparison.Status = compareSyncEntries(comparison)
		comparisons = append(comparisons, comparison)
	}

	return comparisons, nil
}

func (m *Manager) SyncDirectories(tabID string, localPath string, remotePath string, direction string) error {
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction != "upload" && direction != "download" {
		return fmt.Errorf("unsupported sync direction: %s", direction)
	}

	client, err := m.clientForTab(tabID)
	if err != nil {
		return err
	}
	comparisons, err := m.CompareDirectories(tabID, localPath, remotePath)
	if err != nil {
		return err
	}

	localRoot := filepath.Clean(strings.TrimSpace(localPath))
	remoteRoot := normalizeRemoteSyncRoot(remotePath)
	selectedPaths := make([]string, 0, len(comparisons))
	var failures []error
	for _, comparison := range comparisons {
		if comparison.Status == "type-conflict" {
			failures = append(failures, fmt.Errorf("type conflict: %s", comparison.RelativePath))
			continue
		}
		if !isSyncChange(comparison, direction) || hasSelectedSyncParent(selectedPaths, comparison.RelativePath) {
			continue
		}
		selectedPaths = append(selectedPaths, comparison.RelativePath)
	}

	for _, relativePath := range selectedPaths {
		displayPath := relativePath
		if direction == "upload" {
			localTarget := filepath.Join(localRoot, filepath.FromSlash(relativePath))
			remoteTarget := path.Join(remoteRoot, relativePath)
			if err := m.uploadPathWithQueue(client, localTarget, remoteTarget, displayPath); err != nil {
				if !errors.Is(err, transport.ErrTransferCancelled) {
					failures = append(failures, fmt.Errorf("upload %s: %w", relativePath, err))
				}
			}
			continue
		}

		remoteTarget := path.Join(remoteRoot, relativePath)
		localTarget := filepath.Join(localRoot, filepath.FromSlash(relativePath))
		if err := m.downloadPathWithQueue(client, remoteTarget, localTarget, displayPath); err != nil {
			if !errors.Is(err, transport.ErrTransferCancelled) {
				failures = append(failures, fmt.Errorf("download %s: %w", relativePath, err))
			}
		}
	}

	if len(selectedPaths) == 0 && len(failures) == 0 {
		m.addLog("目錄已同步，沒有需要處理的差異", "done")
	}
	return errors.Join(failures...)
}

func (m *Manager) clientForTab(tabID string) (transport.Client, error) {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab not connected")
	}
	return client, nil
}

func collectLocalSyncEntries(root string) (map[string]model.FileEntry, error) {
	entries := make(map[string]model.FileEntry)
	var visit func(string, string) error
	visit = func(currentPath string, currentRelativePath string) error {
		children, err := os.ReadDir(currentPath)
		if err != nil {
			return err
		}
		for _, child := range children {
			childPath := filepath.Join(currentPath, child.Name())
			info, err := child.Info()
			if err != nil {
				return err
			}
			relativePath := filepath.ToSlash(filepath.Join(currentRelativePath, child.Name()))
			entries[relativePath] = model.FileEntry{
				Name:     child.Name(),
				Path:     childPath,
				Size:     info.Size(),
				Modified: info.ModTime().Format("2006-01-02 15:04"),
				IsDir:    info.IsDir(),
				Side:     "local",
			}
			if info.IsDir() {
				if err := visit(childPath, relativePath); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := visit(root, ""); err != nil {
		return nil, err
	}
	return entries, nil
}

func collectRemoteSyncEntries(client transport.Client, root string) (map[string]model.FileEntry, error) {
	entries := make(map[string]model.FileEntry)
	var visit func(string, string) error
	visit = func(currentPath string, currentRelativePath string) error {
		children, err := client.List(currentPath)
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.Name == "." || child.Name == ".." || strings.TrimSpace(child.Name) == "" {
				continue
			}
			relativePath := path.Join(currentRelativePath, child.Name)
			child.Path = path.Join(currentPath, child.Name)
			entries[relativePath] = child
			if child.IsDir {
				if err := visit(child.Path, relativePath); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := visit(root, ""); err != nil {
		return nil, err
	}
	return entries, nil
}

func normalizeRemoteSyncRoot(remotePath string) string {
	trimmed := strings.TrimSpace(remotePath)
	if trimmed == "" {
		return "/"
	}
	return path.Clean(trimmed)
}

func compareSyncEntries(comparison model.FileComparison) string {
	if !comparison.LocalExists {
		return "remote-only"
	}
	if !comparison.RemoteExists {
		return "local-only"
	}
	if comparison.LocalDirectory != comparison.RemoteDirectory {
		return "type-conflict"
	}
	if comparison.LocalDirectory || (comparison.LocalSize == comparison.RemoteSize && comparison.LocalModified == comparison.RemoteModified) {
		return "same"
	}
	return "different"
}

func isSyncChange(comparison model.FileComparison, direction string) bool {
	if direction == "upload" {
		return comparison.Status == "local-only" || comparison.Status == "different"
	}
	return comparison.Status == "remote-only" || comparison.Status == "different"
}

func hasSelectedSyncParent(selectedPaths []string, relativePath string) bool {
	for _, selectedPath := range selectedPaths {
		if relativePath == selectedPath || strings.HasPrefix(relativePath, selectedPath+"/") {
			return true
		}
	}
	return false
}
