package session

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

func (m *Manager) UploadPaths(tabID string, localPaths []string, remoteBase string) error {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not connected")
	}

	var failures []error
	for _, localPath := range localPaths {
		displayPath := filepath.Base(localPath)
		if err := m.uploadPathWithQueue(client, localPath, path.Join(remoteBase, displayPath), displayPath); err != nil {
			if err == transport.ErrTransferCancelled {
				continue
			}
			m.addLog(fmt.Sprintf("拖曳上傳失敗: %s", displayPath), "failed")
			failures = append(failures, err)
			continue
		}
		m.addLog(fmt.Sprintf("拖曳上傳完成: %s", displayPath), "done")
	}

	return errors.Join(failures...)
}

func (m *Manager) UploadPathsWithSite(site model.Site, localPaths []string, remoteBase string) error {
	client, err := newClient(site.Protocol)
	if err != nil {
		m.addLog(fmt.Sprintf("%s 臨時連線初始化失敗: %v", site.Name, err), "failed")
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Connect(site); err != nil {
		m.addLog(fmt.Sprintf("%s 臨時連線失敗: %v", site.Name, err), "failed")
		return err
	}

	resolvedRemoteBase := remoteBase
	currentDir, _ := client.CurrentDir()
	homeDir := currentDir
	if sftpClient, ok := client.(*transport.SFTPClient); ok {
		if resolvedHome := strings.TrimSpace(sftpClient.HomeDir()); resolvedHome != "" {
			homeDir = resolvedHome
		}
	}
	if homeDir != "" {
		resolvedRemoteBase = resolveRemoteBasePath(homeDir, remoteBase)
	} else if currentDir != "" {
		resolvedRemoteBase = resolveRemoteBasePath(currentDir, remoteBase)
	}
	m.addLog(fmt.Sprintf("SSH 拖曳上傳目標目錄: %s", resolvedRemoteBase), "running")

	var failures []error
	for _, localPath := range localPaths {
		displayPath := filepath.Base(localPath)
		targetPath := path.Join(resolvedRemoteBase, displayPath)
		m.addLog(fmt.Sprintf("SSH 拖曳上傳目標檔案: %s", targetPath), "running")
		if err := m.uploadPathWithQueue(client, localPath, targetPath, displayPath); err != nil {
			if err == transport.ErrTransferCancelled {
				continue
			}
			m.addLog(fmt.Sprintf("拖曳上傳失敗: %s -> %s (%v)", displayPath, targetPath, err), "failed")
			failures = append(failures, fmt.Errorf("upload to %s failed: %w", targetPath, err))
			continue
		}
		m.addLog(fmt.Sprintf("拖曳上傳完成: %s", displayPath), "done")
	}

	return errors.Join(failures...)
}

func resolveRemoteBasePath(currentDir string, remoteBase string) string {
	trimmed := strings.TrimSpace(remoteBase)
	if trimmed == "" {
		return currentDir
	}
	if trimmed == "~" {
		return currentDir
	}
	if strings.HasPrefix(trimmed, "~/") {
		return path.Join(currentDir, strings.TrimPrefix(trimmed, "~/"))
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return path.Join(currentDir, trimmed)
}

func (m *Manager) DownloadPaths(tabID string, remotePaths []string, localBase string) error {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not connected")
	}

	var failures []error
	for _, remotePath := range remotePaths {
		displayPath := path.Base(remotePath)
		if err := m.downloadPathWithQueue(client, remotePath, filepath.Join(localBase, displayPath), displayPath); err != nil {
			if err == transport.ErrTransferCancelled {
				continue
			}
			m.addLog(fmt.Sprintf("拖曳下載失敗: %s", displayPath), "failed")
			failures = append(failures, err)
			continue
		}
		m.addLog(fmt.Sprintf("拖曳下載完成: %s", displayPath), "done")
	}

	return errors.Join(failures...)
}
