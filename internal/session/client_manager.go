package session

import (
	"fmt"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (m *Manager) Connect(tab model.Tab) (string, error) {
	client, err := newClient(tab.Protocol)
	if err != nil {
		m.addLog(fmt.Sprintf("%s 連線初始化失敗: %v", tab.Title, err), "failed")
		return "", err
	}

	if err := client.Connect(model.Site{
		ID:            tab.SiteID,
		Name:          tab.Title,
		Protocol:      tab.Protocol,
		Host:          tab.Host,
		Port:          tab.Port,
		Username:      tab.Username,
		Password:      tab.Password,
		PPKPath:       tab.PPKPath,
		PPKPassphrase: tab.PPKPassphrase,
		LocalPath:     tab.LocalPath,
		RemotePath:    tab.RemotePath,
	}); err != nil {
		m.addLog(fmt.Sprintf("%s 連線失敗: %v", tab.Title, err), "failed")
		return "", err
	}

	m.mu.Lock()
	if existing, ok := m.clients[tab.ID]; ok {
		_ = existing.Close()
	}
	m.clients[tab.ID] = client
	m.mu.Unlock()

	currentDir, err := client.CurrentDir()
	if err != nil || currentDir == "" {
		m.addLog(fmt.Sprintf("%s 已連線", tab.Title), "done")
		return tab.RemotePath, nil
	}
	m.addLog(fmt.Sprintf("%s 已連線到 %s", tab.Title, currentDir), "done")
	return currentDir, nil
}

func (m *Manager) Disconnect(tabID string) error {
	m.mu.Lock()
	client, ok := m.clients[tabID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.clients, tabID)
	m.mu.Unlock()
	if err := client.Close(); err != nil {
		m.addLog(fmt.Sprintf("%s 中斷連線失敗: %v", tabID, err), "failed")
		return err
	}
	m.addLog(fmt.Sprintf("%s 已中斷連線", tabID), "done")
	return nil
}

func (m *Manager) ListRemote(tabID string, remotePath string) ([]model.FileEntry, error) {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab not connected: %s", tabID)
	}
	return client.List(remotePath)
}

func (m *Manager) CreateRemoteDirectory(tabID string, remotePath string) error {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not connected")
	}
	if err := client.Mkdir(remotePath); err != nil {
		m.addLog(fmt.Sprintf("建立遠端目錄失敗: %s", remotePath), "failed")
		return err
	}
	m.addLog(fmt.Sprintf("已建立遠端目錄: %s", remotePath), "done")
	return nil
}

func (m *Manager) DeleteRemotePath(tabID string, remotePath string) error {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not connected")
	}
	if err := m.deleteRemotePathRecursive(client, remotePath); err != nil {
		m.addLog(fmt.Sprintf("刪除遠端項目失敗: %s", remotePath), "failed")
		return err
	}
	m.addLog(fmt.Sprintf("已刪除遠端項目: %s", remotePath), "done")
	return nil
}

func (m *Manager) RenameRemotePath(tabID string, oldPath string, newPath string) error {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not connected")
	}
	if err := client.Rename(oldPath, newPath); err != nil {
		m.addLog(fmt.Sprintf("改名遠端項目失敗: %s", oldPath), "failed")
		return err
	}
	m.addLog(fmt.Sprintf("已改名遠端項目: %s", newPath), "done")
	return nil
}
