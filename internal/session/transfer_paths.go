package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

func (m *Manager) uploadPathWithQueue(client transport.Client, localPath, remotePath, displayPath string) error {
	return m.uploadPathWithParent(client, localPath, remotePath, displayPath, "")
}

func (m *Manager) uploadPathWithParent(client transport.Client, localPath, remotePath, displayPath, parentID string) (err error) {
	itemID := m.addChildTransfer(displayPath, "upload", parentID)
	defer func() { m.finishPathTransfer(itemID, err) }()
	if !m.awaitTransferActive(itemID, 0) {
		return transport.ErrTransferCancelled
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := client.Mkdir(remotePath); err != nil && !remoteDirectoryExists(client, remotePath) {
			return err
		}
		entries, err := os.ReadDir(localPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !m.awaitTransferActive(itemID, m.transferProgress(itemID)) {
				return transport.ErrTransferCancelled
			}
			if isHiddenName(entry.Name()) {
				continue
			}
			if err := m.uploadPathWithParent(client, filepath.Join(localPath, entry.Name()), path.Join(remotePath, entry.Name()), filepath.ToSlash(filepath.Join(displayPath, entry.Name())), itemID); err != nil {
				return err
			}
		}
	} else {
		if err := client.Upload(localPath, remotePath, m.pathTransferProgress(itemID)); err != nil {
			return err
		}
	}
	if !m.awaitTransferActive(itemID, m.transferProgress(itemID)) {
		return transport.ErrTransferCancelled
	}
	m.addLog(fmt.Sprintf("已上傳: %s", displayPath), "done")
	return nil
}

func (m *Manager) downloadPathWithQueue(client transport.Client, remotePath, localPath, displayPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(localPath))
	if err != nil {
		return err
	}
	defer root.Close()
	return m.downloadPathWithParent(client, remotePath, root, filepath.Base(localPath), displayPath, "", nil)
}

func (m *Manager) downloadPathWithParent(client transport.Client, remotePath string, root *os.Root, relativePath, displayPath, parentID string, knownEntry *model.FileEntry) (err error) {
	itemID := m.addChildTransfer(displayPath, "download", parentID)
	defer func() { m.finishPathTransfer(itemID, err) }()
	if !m.awaitTransferActive(itemID, 0) {
		return transport.ErrTransferCancelled
	}
	var entry model.FileEntry
	if knownEntry != nil {
		entry = *knownEntry
	} else {
		entry, err = client.Stat(remotePath)
		if err != nil {
			return err
		}
	}
	if entry.IsDir {
		entries, err := client.List(remotePath)
		if err != nil {
			return err
		}
		// Validate the complete listing before creating anything from that response.
		for _, child := range entries {
			if err := transport.ValidateEntryName(child.Name); err != nil {
				return err
			}
		}
		if err := root.MkdirAll(relativePath, 0o755); err != nil {
			return err
		}
		for _, child := range entries {
			if !m.awaitTransferActive(itemID, m.transferProgress(itemID)) {
				return transport.ErrTransferCancelled
			}
			if err := m.downloadPathWithParent(client, path.Join(remotePath, child.Name), root, filepath.Join(relativePath, child.Name), filepath.ToSlash(filepath.Join(displayPath, child.Name)), itemID, &child); err != nil {
				return err
			}
		}
	} else {
		if err := downloadFile(root, relativePath, func(destination io.Writer) error {
			if !m.awaitTransferActive(itemID, 0) {
				return transport.ErrTransferCancelled
			}
			if err := client.Download(remotePath, destination, m.pathTransferProgress(itemID)); err != nil {
				return err
			}
			// Cancellation or pause arriving with the final bytes must be observed before
			// replacing the user's existing file.
			if !m.awaitTransferActive(itemID, m.transferProgress(itemID)) {
				return transport.ErrTransferCancelled
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if !m.awaitTransferActive(itemID, m.transferProgress(itemID)) {
		return transport.ErrTransferCancelled
	}
	m.addLog(fmt.Sprintf("已下載: %s", displayPath), "done")
	return nil
}

func (m *Manager) pathTransferProgress(itemID string) func(int64, int64, int64) bool {
	return func(transferred, total, speedBps int64) bool {
		progress := 0
		if total > 0 {
			progress = int((transferred * 100) / total)
		}
		if !m.awaitTransferActive(itemID, progress) {
			return false
		}
		m.updateTransfer(itemID, progress, speedBps, "running")
		return !m.isTransferCancelled(itemID)
	}
}

func (m *Manager) finishPathTransfer(itemID string, err error) {
	if errors.Is(err, transport.ErrTransferCancelled) || m.isTransferCancelled(itemID) {
		m.updateTransfer(itemID, m.transferProgress(itemID), 0, "cancelled")
	} else if err != nil {
		m.updateTransfer(itemID, m.transferProgress(itemID), 0, "failed")
		m.addLog(fmt.Sprintf("傳輸失敗: %v", err), "failed")
	} else {
		m.updateTransfer(itemID, 100, 0, "done")
	}
}

func remoteDirectoryExists(client transport.Client, remotePath string) bool {
	if strings.TrimSpace(remotePath) == "" {
		return false
	}
	entry, err := client.Stat(remotePath)
	return err == nil && entry.IsDir
}

func (m *Manager) deleteRemotePathRecursive(client transport.Client, remotePath string) error {
	entry, err := client.Stat(remotePath)
	if err != nil {
		return err
	}
	if entry.IsDir {
		entries, err := client.List(remotePath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := transport.ValidateEntryName(entry.Name); err != nil {
				return err
			}
		}
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
