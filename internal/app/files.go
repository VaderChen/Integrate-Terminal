package app

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"IntegTERM/internal/model"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) UploadDroppedPaths(tabID string, localPaths []string, remoteBase string) error {
	a.markTabActivity(tabID)
	return a.sessionManager.UploadPaths(tabID, localPaths, remoteBase)
}

func (a *App) UploadDroppedPathsToSite(site model.Site, localPaths []string, remoteBase string) error {
	site.Protocol = "sftp"
	return a.sessionManager.UploadPathsWithSite(site, localPaths, remoteBase)
}

func (a *App) DownloadDroppedPaths(tabID string, remotePaths []string, localBase string) error {
	a.markTabActivity(tabID)
	return a.sessionManager.DownloadPaths(tabID, remotePaths, localBase)
}

func (a *App) CreateDirectory(tabID string, side string, basePath string, name string) error {
	a.markTabActivity(tabID)
	trimmedName := strings.TrimSpace(strings.Trim(name, "/"))
	if trimmedName == "" {
		return fmt.Errorf("directory name is required")
	}

	switch side {
	case "local":
		targetPath := filepath.Join(basePath, trimmedName)
		if err := os.MkdirAll(targetPath, 0o755); err != nil {
			a.sessionManager.AppendLog(fmt.Sprintf("建立本機目錄失敗: %s", targetPath), "failed")
			return err
		}
		a.sessionManager.AppendLog(fmt.Sprintf("已建立本機目錄: %s", targetPath), "done")
		return nil
	case "remote":
		return a.sessionManager.CreateRemoteDirectory(tabID, path.Join(basePath, trimmedName))
	default:
		return fmt.Errorf("unsupported side: %s", side)
	}
}

func (a *App) DeleteEntry(tabID string, side string, targetPath string) error {
	if err := a.validateDeleteTarget(tabID, side, targetPath); err != nil {
		return err
	}
	a.markTabActivity(tabID)
	switch side {
	case "local":
		if err := os.RemoveAll(targetPath); err != nil {
			a.sessionManager.AppendLog(fmt.Sprintf("刪除本機項目失敗: %s", targetPath), "failed")
			return err
		}
		a.sessionManager.AppendLog(fmt.Sprintf("已刪除本機項目: %s", targetPath), "done")
		return nil
	case "remote":
		return a.sessionManager.DeleteRemotePath(tabID, targetPath)
	default:
		return fmt.Errorf("unsupported side: %s", side)
	}
}

func (a *App) DeleteEntries(tabID string, side string, targetPaths []string) error {
	for _, target := range targetPaths {
		if err := a.validateDeleteTarget(tabID, side, target); err != nil {
			return err
		}
	}
	for _, targetPath := range collapseNestedDeleteTargets(side, targetPaths) {
		if err := a.DeleteEntry(tabID, side, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func collapseNestedDeleteTargets(side string, targetPaths []string) []string {
	cleaned := make([]string, 0, len(targetPaths))
	seen := make(map[string]struct{}, len(targetPaths))
	for _, targetPath := range targetPaths {
		trimmed := strings.TrimSpace(targetPath)
		if trimmed == "" {
			continue
		}
		if side == "remote" {
			trimmed = path.Clean(trimmed)
		} else {
			trimmed = filepath.Clean(trimmed)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}

	sort.Slice(cleaned, func(i, j int) bool {
		return len(cleaned[i]) < len(cleaned[j])
	})

	collapsed := make([]string, 0, len(cleaned))
	for _, candidate := range cleaned {
		skip := false
		for _, kept := range collapsed {
			if deleteTargetContains(side, kept, candidate) {
				skip = true
				break
			}
		}
		if !skip {
			collapsed = append(collapsed, candidate)
		}
	}

	return collapsed
}

func deleteTargetContains(side string, parent string, child string) bool {
	if parent == child {
		return true
	}
	if side == "remote" {
		prefix := strings.TrimSuffix(path.Clean(parent), "/") + "/"
		return strings.HasPrefix(path.Clean(child), prefix)
	}
	prefix := strings.TrimSuffix(filepath.Clean(parent), string(os.PathSeparator)) + string(os.PathSeparator)
	return strings.HasPrefix(filepath.Clean(child), prefix)
}

func (a *App) RenameEntry(tabID string, side string, sourcePath string, newName string) error {
	a.markTabActivity(tabID)
	trimmedName := strings.TrimSpace(strings.Trim(newName, "/"))
	if trimmedName == "" {
		return fmt.Errorf("new name is required")
	}

	switch side {
	case "local":
		targetPath := filepath.Join(filepath.Dir(sourcePath), trimmedName)
		if err := os.Rename(sourcePath, targetPath); err != nil {
			a.sessionManager.AppendLog(fmt.Sprintf("改名本機項目失敗: %s", sourcePath), "failed")
			return err
		}
		a.sessionManager.AppendLog(fmt.Sprintf("已改名本機項目: %s", targetPath), "done")
		return nil
	case "remote":
		targetPath := path.Join(path.Dir(sourcePath), trimmedName)
		return a.sessionManager.RenameRemotePath(tabID, sourcePath, targetPath)
	default:
		return fmt.Errorf("unsupported side: %s", side)
	}
}

func (a *App) MoveEntriesToDirectory(tabID string, side string, sourcePaths []string, targetDirectory string) error {
	a.markTabActivity(tabID)
	if strings.TrimSpace(targetDirectory) == "" {
		return fmt.Errorf("target directory is required")
	}
	if len(sourcePaths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(sourcePaths))
	for _, sourcePath := range sourcePaths {
		trimmedSource := strings.TrimSpace(sourcePath)
		if trimmedSource == "" {
			continue
		}
		if _, ok := seen[trimmedSource]; ok {
			continue
		}
		seen[trimmedSource] = struct{}{}

		switch side {
		case "local":
			targetPath := filepath.Join(targetDirectory, filepath.Base(trimmedSource))
			if targetPath == trimmedSource {
				continue
			}
			if err := os.Rename(trimmedSource, targetPath); err != nil {
				a.sessionManager.AppendLog(fmt.Sprintf("移動本機項目失敗: %s", trimmedSource), "failed")
				return err
			}
			a.sessionManager.AppendLog(fmt.Sprintf("已移動本機項目: %s", targetPath), "done")
		case "remote":
			targetPath := path.Join(targetDirectory, path.Base(trimmedSource))
			if targetPath == trimmedSource {
				continue
			}
			if err := a.sessionManager.RenameRemotePath(tabID, trimmedSource, targetPath); err != nil {
				a.sessionManager.AppendLog(fmt.Sprintf("移動遠端項目失敗: %s", trimmedSource), "failed")
				return err
			}
		default:
			return fmt.Errorf("unsupported side: %s", side)
		}
	}

	return nil
}

func (a *App) OpenLocalPath(targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("path is required")
	}
	if _, err := os.Stat(targetPath); err != nil {
		return err
	}

	command, args, err := openCommandForPath(targetPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("開啟本機項目失敗: %s", targetPath), "failed")
		return err
	}
	_ = cmd.Process.Release()
	a.sessionManager.AppendLog(fmt.Sprintf("已開啟本機項目: %s", targetPath), "done")
	return nil
}

func (a *App) ExecuteLocalPath(targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("path is required")
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("cannot execute a directory")
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("file is not executable")
	}

	cmd := exec.Command(targetPath)
	cmd.Dir = filepath.Dir(targetPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("執行本機檔案失敗: %s", targetPath), "failed")
		return err
	}
	_ = cmd.Process.Release()
	a.sessionManager.AppendLog(fmt.Sprintf("已執行本機檔案: %s", targetPath), "done")
	return nil
}

func (a *App) SelectPPKFile() (string, error) {
	ctx := a.appContext()
	if ctx == nil {
		return "", nil
	}

	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "選擇 PPK 金鑰檔",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "PuTTY Private Key",
				Pattern:     "*.ppk",
			},
		},
	})
}

func (a *App) SelectDirectory() (string, error) {
	ctx := a.appContext()
	if ctx == nil {
		return "", nil
	}

	return wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "選擇下載目錄",
	})
}
