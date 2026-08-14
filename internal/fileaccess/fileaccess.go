package fileaccess

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

"github.com/VaderChen/Integrate-Terminal/internal/macsecurity"
)

const registryFileName = "file-access.json"

type registry struct {
	Directories map[string]string `json:"directories"`
}

var (
	mu                sync.Mutex
	baseDir           string
	activeDirectories = map[string]*macsecurity.Access{}
)

func Init(dir string) {
	mu.Lock()
	defer mu.Unlock()

	releaseAllLocked()
	baseDir = dir
	if !macsecurity.Available() {
		return
	}

	current := loadLocked()
	paths := make([]string, 0, len(current.Directories))
	for path := range current.Directories {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) < len(paths[j])
	})

	changed := false
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if activeParentLocked(cleaned) != "" {
			delete(current.Directories, path)
			changed = true
			continue
		}

		bookmark := current.Directories[path]
		access, err := macsecurity.ResolveBookmark(bookmark)
		if err != nil {
			continue
		}
		resolvedPath := filepath.Clean(access.Path)
		if activeParentLocked(resolvedPath) != "" {
			access.Release()
			delete(current.Directories, path)
			changed = true
			continue
		}
		activeDirectories[resolvedPath] = access
		if resolvedPath != path {
			delete(current.Directories, path)
			current.Directories[resolvedPath] = bookmark
			changed = true
		}
		if access.Stale {
			if refreshed, err := macsecurity.CreateBookmark(access.Path); err == nil {
				current.Directories[resolvedPath] = refreshed
				changed = true
			}
		}
	}

	if changed {
		_ = saveLocked(current)
	}
}

func RememberDirectory(path string) error {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "." || cleaned == "" {
		return errors.New("授權目錄不可為空")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return fmt.Errorf("無法讀取授權目錄: %w", err)
	}
	if !info.IsDir() {
		return errors.New("授權路徑不是目錄")
	}
	if !macsecurity.Available() {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	if baseDir == "" {
		return errors.New("檔案授權服務尚未初始化")
	}
	if activeParentLocked(cleaned) != "" {
		return nil
	}

	bookmark, err := macsecurity.CreateBookmark(cleaned)
	if err != nil {
		return fmt.Errorf("無法建立持續性目錄授權: %w", err)
	}
	access, err := macsecurity.ResolveBookmark(bookmark)
	if err != nil {
		return fmt.Errorf("無法啟用持續性目錄授權: %w", err)
	}

	current := loadLocked()
	descendants := make([]string, 0)
	for authorizedPath := range current.Directories {
		if authorizedPath != cleaned && contains(cleaned, authorizedPath) {
			descendants = append(descendants, authorizedPath)
			delete(current.Directories, authorizedPath)
		}
	}
	current.Directories[cleaned] = bookmark
	if err := saveLocked(current); err != nil {
		access.Release()
		return fmt.Errorf("無法保存持續性目錄授權: %w", err)
	}

	for _, descendant := range descendants {
		if active := activeDirectories[descendant]; active != nil {
			active.Release()
		}
		delete(activeDirectories, descendant)
	}
	activeDirectories[cleaned] = access
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	releaseAllLocked()
}

func loadLocked() registry {
	current := registry{Directories: map[string]string{}}
	if baseDir == "" {
		return current
	}
	data, err := os.ReadFile(filepath.Join(baseDir, registryFileName))
	if err != nil || json.Unmarshal(data, &current) != nil || current.Directories == nil {
		return registry{Directories: map[string]string{}}
	}
	return current
}

func saveLocked(current registry) error {
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(baseDir, registryFileName), data, 0o600)
}

func activeParentLocked(path string) string {
	for authorizedPath := range activeDirectories {
		if contains(authorizedPath, path) {
			return authorizedPath
		}
	}
	return ""
}

func contains(parent string, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func releaseAllLocked() {
	for path, access := range activeDirectories {
		access.Release()
		delete(activeDirectories, path)
	}
}
