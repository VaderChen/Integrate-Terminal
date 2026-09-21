package session

import (
	"fmt"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

func (m *Manager) Connect(tab model.Tab) (string, error) {
	connection, err := m.PrepareConnection(tab)
	if err != nil {
		return "", err
	}
	cleanup := m.CommitConnection(tab.ID, connection)
	cleanup()
	return connection.RemotePath, nil
}

// PreparedConnection 在提交前不會取代現有連線，可安全取消過期的連線請求。
type PreparedConnection struct {
	client     transport.Client
	RemotePath string
}

func (c *PreparedConnection) Close() error { return c.client.Close() }

func (m *Manager) PrepareConnection(tab model.Tab) (*PreparedConnection, error) {
	client, err := newClient(tab.Protocol)
	if err != nil {
		m.addLog(fmt.Sprintf("%s 連線初始化失敗: %v", tab.Title, err), "failed")
		return nil, err
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
		_ = client.Close()
		m.addLog(fmt.Sprintf("%s 連線失敗: %v", tab.Title, err), "failed")
		return nil, err
	}

	currentDir, err := client.CurrentDir()
	if err != nil || currentDir == "" {
		m.addLog(fmt.Sprintf("%s 已連線", tab.Title), "done")
		currentDir = tab.RemotePath
	} else {
		m.addLog(fmt.Sprintf("%s 已連線到 %s", tab.Title, currentDir), "done")
	}
	return &PreparedConnection{client: client, RemotePath: currentDir}, nil
}

// CommitConnection 只更新索引；呼叫端釋放狀態鎖後再執行傳回的清理函式。
func (m *Manager) CommitConnection(tabID string, connection *PreparedConnection) func() {
	m.mu.Lock()
	existing := m.clients[tabID]
	m.clients[tabID] = connection.client
	m.mu.Unlock()
	return func() {
		if existing != nil {
			_ = existing.Close()
		}
	}
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
		return nil, fmt.Errorf("tab not connected")
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
