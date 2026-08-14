package app

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/fileaccess"
	"github.com/VaderChen/Integrate-Terminal/internal/keystore"
	"github.com/VaderChen/Integrate-Terminal/internal/model"

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
	prefix := filepath.Clean(parent) + string(os.PathSeparator)
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
	cmd, err := executableCommand(targetPath, info)
	if err != nil {
		return err
	}
	cmd.Dir = filepath.Dir(targetPath)
	configureDetachedCommand(cmd)
	if err := cmd.Start(); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("執行本機檔案失敗: %s", targetPath), "failed")
		return err
	}
	_ = cmd.Process.Release()
	a.sessionManager.AppendLog(fmt.Sprintf("已執行本機檔案: %s", targetPath), "done")
	return nil
}

func (a *App) SelectPPKFile() (string, error) {
	if a.ctx == nil {
		return "", nil
	}

	selectedPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "選擇 PPK 金鑰檔",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "PuTTY Private Key",
				Pattern:     "*.ppk",
			},
		},
	})
	if err != nil || selectedPath == "" {
		return selectedPath, err
	}

	// 保留 bookmark 與副本，讓舊沙盒版本選取的金鑰可延續使用。
	// 必須立刻建立 security-scoped bookmark 與容器副本，否則下次啟動就讀不到了。
	if err := keystore.Remember(selectedPath); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("登記金鑰檔失敗: %v", err), "failed")
		return "", fmt.Errorf("無法登記金鑰檔，請改選其他位置的檔案: %w", err)
	}

	return selectedPath, nil
}

func (a *App) SelectDirectory() (string, error) {
	if a.ctx == nil {
		return "", nil
	}

	selectedDirectory, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "選擇並授權本機目錄",
	})
	if err != nil || selectedDirectory == "" {
		return selectedDirectory, err
	}
	if err := fileaccess.RememberDirectory(selectedDirectory); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("保存本機目錄授權失敗: %v", err), "failed")
		return "", err
	}

	a.sessionManager.AppendLog(fmt.Sprintf("已授權本機目錄及其所有子目錄: %s", selectedDirectory), "done")
	return selectedDirectory, nil
}

// AuthorizeKeyDirectory 讓使用者授權一整個金鑰資料夾。
//
// 舊沙盒版本保存的 PPK 絕對路徑可能需要重新授權。
// 逐一重選每個金鑰很繁瑣，改為選一次資料夾即可涵蓋其下所有金鑰檔。
func (a *App) AuthorizeKeyDirectory(suggestedPath string) (string, error) {
	if a.ctx == nil {
		return "", nil
	}

	// 直接把對話框開在金鑰所在資料夾，使用者不必自己找。
	defaultDirectory := strings.TrimSpace(suggestedPath)
	if defaultDirectory != "" {
		if info, err := os.Stat(defaultDirectory); err != nil || !info.IsDir() {
			defaultDirectory = filepath.Dir(defaultDirectory)
		}
	}

	selectedDir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "選擇存放 PPK 金鑰的資料夾",
		DefaultDirectory: defaultDirectory,
	})
	if err != nil || selectedDir == "" {
		return selectedDir, err
	}

	if err := fileaccess.RememberDirectory(selectedDir); err != nil {
		a.sessionManager.AppendLog(fmt.Sprintf("授權金鑰資料夾失敗: %v", err), "failed")
		return "", fmt.Errorf("授權金鑰資料夾失敗: %w", err)
	}

	a.sessionManager.AppendLog(fmt.Sprintf("已授權金鑰資料夾: %s", selectedDir), "done")
	return selectedDir, nil
}

// PendingKeyAuthorizations 回報哪些站台的金鑰目前讀不到，需要使用者授權。
// 前端可用它提示使用者，而不必等到連線失敗才發現。
func (a *App) PendingKeyAuthorizations() []string {
	sites, err := a.store.LoadSites()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	pending := make([]string, 0)
	for _, site := range sites {
		path := strings.TrimSpace(site.PPKPath)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if keystore.NeedsReselect(path) {
			pending = append(pending, path)
		}
	}
	return pending
}
