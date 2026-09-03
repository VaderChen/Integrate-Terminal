package session

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

func (m *Manager) uploadPathWithQueue(client transport.Client, localPath string, remotePath string, displayPath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	itemID := m.addTransfer(displayPath, "upload")
	if m.isTransferCancelled(itemID) {
		m.updateTransfer(itemID, 0, 0, "cancelled")
		return transport.ErrTransferCancelled
	}

	if info.IsDir() {
		if existing, statErr := client.Stat(remotePath); statErr == nil && !existing.IsDir {
			m.updateTransfer(itemID, 100, 0, "failed")
			return fmt.Errorf("remote path is a file: %s", remotePath)
		}
		if err := client.Mkdir(remotePath); err != nil && !os.IsExist(err) {
			// Some servers return an error when the folder already exists.
			if !isRemoteExistsError(err) && !remoteDirectoryExists(client, remotePath) {
				m.updateTransfer(itemID, 100, 0, "failed")
				m.addLog(fmt.Sprintf("建立遠端資料夾失敗: %s", displayPath), "failed")
				return err
			}
		}

		entries, err := os.ReadDir(localPath)
		if err != nil {
			m.updateTransfer(itemID, 100, 0, "failed")
			m.addLog(fmt.Sprintf("讀取本機資料夾失敗: %s", displayPath), "failed")
			return err
		}
		for _, entry := range entries {
			if isHiddenName(entry.Name()) {
				continue
			}
			childLocalPath := filepath.Join(localPath, entry.Name())
			childRemotePath := path.Join(remotePath, entry.Name())
			childDisplayPath := filepath.ToSlash(filepath.Join(displayPath, entry.Name()))
			if err := m.uploadPathWithQueue(client, childLocalPath, childRemotePath, childDisplayPath); err != nil {
				if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
					m.updateTransfer(itemID, m.transferProgress(itemID), 0, "cancelled")
					return transport.ErrTransferCancelled
				}
				m.updateTransfer(itemID, 100, 0, "failed")
				return err
			}
		}
		m.updateTransfer(itemID, 100, 0, "done")
		m.addLog(fmt.Sprintf("已同步資料夾: %s", displayPath), "done")
		return nil
	}

	if existing, statErr := client.Stat(remotePath); statErr == nil {
		_, conflictStrategy := m.transferPolicy()
		switch conflictStrategy {
		case "skip":
			m.updateTransfer(itemID, 100, 0, "done")
			m.addLog(fmt.Sprintf("已略過遠端既有檔案: %s", displayPath), "done")
			return nil
		case "fail":
			m.updateTransfer(itemID, 100, 0, "failed")
			return fmt.Errorf("remote file already exists: %s", existing.Path)
		}
	}

	if err := m.uploadFileWithRetry(client, itemID, localPath, remotePath); err != nil {
		if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
			m.updateTransfer(itemID, m.transferProgress(itemID), 0, "cancelled")
			return nil
		}
		m.updateTransfer(itemID, 100, 0, "failed")
		m.addLog(fmt.Sprintf("上傳檔案失敗: %s", displayPath), "failed")
		return err
	}

	m.updateTransfer(itemID, 100, 0, "done")
	m.addLog(fmt.Sprintf("已上傳檔案: %s", displayPath), "done")
	return nil
}

func (m *Manager) uploadFileWithRetry(client transport.Client, itemID string, localPath string, remotePath string) error {
	retryCount, _ := m.transferPolicy()
	maxAttempts := retryCount + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		m.updateTransferAttempt(itemID, attempt, maxAttempts, "")
		err := client.Upload(localPath, remotePath, func(transferred int64, total int64, speedBps int64) bool {
			progress := 0
			if total > 0 {
				progress = int((transferred * 100) / total)
			}
			if !m.awaitTransferActive(itemID, progress) {
				return false
			}
			m.updateTransfer(itemID, progress, speedBps, "running")
			return !m.isTransferCancelled(itemID)
		})
		if err == nil {
			return nil
		}
		if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
			return err
		}
		lastErr = err
		m.updateTransferAttempt(itemID, attempt, maxAttempts, err.Error())
		if attempt < maxAttempts {
			m.addLog(fmt.Sprintf("上傳失敗，%d 秒後重試 (%d/%d): %s", attempt, attempt, maxAttempts, filepath.Base(localPath)), "running")
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return lastErr
}

func (m *Manager) downloadPathWithQueue(client transport.Client, remotePath string, localPath string, displayPath string) error {
	itemID := m.addTransfer(displayPath, "download")
	if m.isTransferCancelled(itemID) {
		m.updateTransfer(itemID, 0, 0, "cancelled")
		return transport.ErrTransferCancelled
	}

	entries, err := client.List(remotePath)
	if err == nil {
		if err := os.MkdirAll(localPath, 0o755); err != nil {
			m.updateTransfer(itemID, 100, 0, "failed")
			m.addLog(fmt.Sprintf("建立本機資料夾失敗: %s", displayPath), "failed")
			return err
		}
		for _, entry := range entries {
			childRemotePath := path.Join(remotePath, entry.Name)
			childLocalPath := filepath.Join(localPath, entry.Name)
			childDisplayPath := filepath.ToSlash(filepath.Join(displayPath, entry.Name))
			if err := m.downloadPathWithQueue(client, childRemotePath, childLocalPath, childDisplayPath); err != nil {
				if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
					m.updateTransfer(itemID, m.transferProgress(itemID), 0, "cancelled")
					return transport.ErrTransferCancelled
				}
				m.updateTransfer(itemID, 100, 0, "failed")
				return err
			}
		}
		m.updateTransfer(itemID, 100, 0, "done")
		m.addLog(fmt.Sprintf("已同步資料夾: %s", displayPath), "done")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		m.updateTransfer(itemID, 100, 0, "failed")
		m.addLog(fmt.Sprintf("建立本機路徑失敗: %s", displayPath), "failed")
		return err
	}

	if existing, statErr := os.Stat(localPath); statErr == nil {
		_, conflictStrategy := m.transferPolicy()
		switch conflictStrategy {
		case "skip":
			m.updateTransfer(itemID, 100, 0, "done")
			m.addLog(fmt.Sprintf("已略過本機既有檔案: %s", displayPath), "done")
			return nil
		case "fail":
			m.updateTransfer(itemID, 100, 0, "failed")
			return fmt.Errorf("local path already exists: %s", localPath)
		case "overwrite":
			if existing.IsDir() {
				m.updateTransfer(itemID, 100, 0, "failed")
				return fmt.Errorf("local path is a directory: %s", localPath)
			}
		}
	}

	if err := m.downloadFileWithRetry(client, itemID, remotePath, localPath); err != nil {
		if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
			_ = os.Remove(localPath)
			m.updateTransfer(itemID, m.transferProgress(itemID), 0, "cancelled")
			return nil
		}
		m.updateTransfer(itemID, 100, 0, "failed")
		m.addLog(fmt.Sprintf("下載檔案失敗: %s", displayPath), "failed")
		return err
	}

	m.updateTransfer(itemID, 100, 0, "done")
	m.addLog(fmt.Sprintf("已下載檔案: %s", displayPath), "done")
	return nil
}

func (m *Manager) downloadFileWithRetry(client transport.Client, itemID string, remotePath string, localPath string) error {
	retryCount, _ := m.transferPolicy()
	maxAttempts := retryCount + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			_ = os.Remove(localPath)
		}
		m.updateTransferAttempt(itemID, attempt, maxAttempts, "")
		err := client.Download(remotePath, localPath, func(transferred int64, total int64, speedBps int64) bool {
			progress := 0
			if total > 0 {
				progress = int((transferred * 100) / total)
			}
			if !m.awaitTransferActive(itemID, progress) {
				return false
			}
			m.updateTransfer(itemID, progress, speedBps, "running")
			return !m.isTransferCancelled(itemID)
		})
		if err == nil {
			return nil
		}
		if err == transport.ErrTransferCancelled || m.isTransferCancelled(itemID) {
			return err
		}
		lastErr = err
		m.updateTransferAttempt(itemID, attempt, maxAttempts, err.Error())
		if attempt < maxAttempts {
			m.addLog(fmt.Sprintf("下載失敗，%d 秒後重試 (%d/%d): %s", attempt, attempt, maxAttempts, filepath.Base(localPath)), "running")
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return lastErr
}

func isRemoteExistsError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "file exists") ||
		strings.Contains(message, "already exists") ||
		strings.Contains(message, "failure") ||
		strings.Contains(message, "550")
}

func remoteDirectoryExists(client transport.Client, remotePath string) bool {
	if strings.TrimSpace(remotePath) == "" {
		return false
	}
	_, err := client.List(remotePath)
	return err == nil
}

func (m *Manager) deleteRemotePathRecursive(client transport.Client, remotePath string) error {
	entries, err := client.List(remotePath)
	if err == nil {
		for _, entry := range entries {
			childPath := path.Join(remotePath, entry.Name)
			if entry.IsDir {
				if err := m.deleteRemotePathRecursive(client, childPath); err != nil {
					return err
				}
				continue
			}
			if err := client.Remove(childPath); err != nil {
				return err
			}
		}
	}

	if err := client.Remove(remotePath); err == nil {
		return nil
	}

	if remover, ok := client.(interface{ RemoveDir(string) error }); ok {
		if err := remover.RemoveDir(remotePath); err == nil {
			return nil
		}
	}

	return fmt.Errorf("delete remote path failed: %s", remotePath)
}
