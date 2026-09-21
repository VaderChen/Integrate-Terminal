package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/session"
)

func receiveWithin[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("操作被等待中的連線阻塞")
		var zero T
		return zero
	}
}

// 暫停 FTP greeting，讓測試可確定連線仍在等待網路回應。
func delayedFTP(t *testing.T, serve func(net.Conn)) (model.Site, <-chan struct{}, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var connection net.Conn
	release := func() { once.Do(func() { close(resume) }) }
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		mu.Lock()
		connection = conn
		mu.Unlock()
		close(accepted)
		<-resume
		serve(conn)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		release()
		// Accept 若剛好成功，先等它公布 connection 再關閉。
		select {
		case <-accepted:
			mu.Lock()
			_ = connection.Close()
			mu.Unlock()
		case <-done:
		}
		<-done
	})
	site := model.Site{Protocol: "ftp", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "fixture", Password: "fixture", RemotePath: "/"}
	return site, accepted, release
}

func TestPendingConnectionDoesNotBlockExistingTerminal(t *testing.T) {
	for _, entry := range []string{"file", "ssh", "rest-file", "rest-ssh"} {
		t.Run(entry, func(t *testing.T) {
			a := regressionApp(t, nil)
			a.tabs = []model.Tab{{ID: "existing", Mode: "terminal", SessionID: "existing-session"}}
			serve := serveMCPFTPControl
			if entry == "ssh" || entry == "rest-ssh" {
				serve = func(net.Conn) {}
			}
			site, accepted, release := delayedFTP(t, serve)
			done := make(chan error, 1)
			go func() {
				var err error
				switch entry {
				case "file":
					_, err = a.CreateTab(site)
				case "ssh":
					_, err = a.CreateSSHTab(site)
				default:
					endpoint := "/api/tabs/file"
					if entry == "rest-ssh" {
						endpoint = "/api/tabs/ssh"
					}
					payload, _ := json.Marshal(map[string]any{"site": site})
					reply := httptest.NewRecorder()
					a.restMux().ServeHTTP(reply, authorizedRequest(http.MethodPost, endpoint, bytes.NewReader(payload)))
					if reply.Code != http.StatusOK {
						err = fmt.Errorf("%s", reply.Body.String())
					}
				}
				done <- err
			}()
			receiveWithin(t, accepted)
			responsive := make(chan struct{})
			go func() {
				_ = a.WriteSSHInput("existing-session", "test")
				_, _ = a.GetTabs()
				close(responsive)
			}()
			receiveWithin(t, responsive)
			release()
			err := receiveWithin(t, done)
			if entry == "file" || entry == "rest-file" {
				if err != nil {
					t.Fatal(err)
				}
				tabs, _ := a.GetTabs()
				if len(tabs) != 2 || !tabs[1].Connected || tabs[1].Hidden != (entry == "rest-file") {
					t.Fatalf("分頁提交不正確: %+v", tabs)
				}
			} else if err == nil {
				t.Fatal("FTP fixture 不應接受 SSH 登入")
			}
		})
	}
}

func TestPendingCreationReservesFreePlanSlot(t *testing.T) {
	a := regressionApp(t, nil)
	a.tabs = []model.Tab{{ID: "existing"}}
	site, accepted, release := delayedFTP(t, serveMCPFTPControl)
	done := make(chan error, 1)
	go func() { _, err := a.CreateTab(site); done <- err }()
	receiveWithin(t, accepted)
	second := make(chan error, 1)
	go func() { _, err := a.CreateTab(site); second <- err }()
	if receiveWithin(t, second) == nil {
		t.Fatal("並行建立分頁超過免費名額")
	}
	release()
	if err := receiveWithin(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestPendingReconnectCannotReviveClosedOrDisconnectedTab(t *testing.T) {
	for _, closeTab := range []bool{false, true} {
		t.Run(fmt.Sprintf("close=%t", closeTab), func(t *testing.T) {
			a := regressionApp(t, nil)
			site, accepted, release := delayedFTP(t, serveMCPFTPControl)
			tab := session.MakeTab(site)
			a.tabs = []model.Tab{tab}
			done := make(chan error, 1)
			go func() { _, err := a.Connect(tab.ID); done <- err }()
			receiveWithin(t, accepted)
			cancelled := make(chan error, 1)
			go func() {
				var err error
				if closeTab {
					_, err = a.CloseTab(tab.ID)
				} else {
					_, err = a.Disconnect(tab.ID)
				}
				cancelled <- err
			}()
			if err := receiveWithin(t, cancelled); err != nil {
				t.Fatal(err)
			}
			release()
			if receiveWithin(t, done) == nil {
				t.Fatal("已取消的連線仍成功提交")
			}
			tabs, _ := a.GetTabs()
			if a.sessionManager.IsConnected(tab.ID) || (closeTab && len(tabs) != 0) || (!closeTab && tabs[0].Connected) {
				t.Fatal("過期的連線重新啟用了分頁")
			}
		})
	}
}

func runningRESTApp(t *testing.T) (*App, *httptest.Server) {
	t.Helper()
	a := regressionApp(t, nil)
	server := httptest.NewServer(a.restMux())
	t.Cleanup(server.Close)
	t.Cleanup(func() { _ = a.applyRESTServerShutdown() })
	a.config.RESTServerEnabled = true
	a.config.RESTServerPort = server.Listener.Addr().(*net.TCPAddr).Port
	a.config.ShowTrayIcon = true
	a.restServer, a.restServerURL = server.Config, server.URL
	if err := a.store.SaveConfig(a.config); err != nil {
		t.Fatal(err)
	}
	return a, server
}

func TestOccupiedRESTPortPreservesServerAndConfig(t *testing.T) {
	for _, entry := range []string{"direct", "http", "attached"} {
		t.Run(entry, func(t *testing.T) {
			service, server := runningRESTApp(t)
			a := service
			if entry == "attached" {
				a = regressionApp(t, service.store)
				a.allowRESTAttach, a.restAttached, a.restServerURL = true, true, server.URL
			}
			occupied, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer occupied.Close()
			previous := a.GetConfig()
			next := previous
			next.RESTServerPort = occupied.Addr().(*net.TCPAddr).Port
			if entry == "http" {
				payload, _ := json.Marshal(configEnvelope{Config: next})
				reply := httptest.NewRecorder()
				a.restMux().ServeHTTP(reply, authorizedRequest(http.MethodPut, "/api/config", bytes.NewReader(payload)))
				if reply.Code == http.StatusOK {
					t.Fatal("切換失敗卻回報成功")
				}
			} else if _, err := a.SaveConfig(next); err == nil {
				t.Fatal("占用中的埠未回傳錯誤")
			}
			saved, err := a.store.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if saved.RESTServerPort != previous.RESTServerPort || a.GetConfig().RESTServerPort != previous.RESTServerPort {
				t.Fatal("失敗的埠設定覆寫了原設定")
			}
			if !detectExistingRESTServer(server.URL) || a.GetRESTServerStatus().BaseURL != server.URL {
				t.Fatal("新埠被占用時原 REST 服務中斷")
			}
		})
	}
}

func TestRESTPortChangePublishesNewListener(t *testing.T) {
	a, old := runningRESTApp(t)
	available, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	next := a.GetConfig()
	next.RESTServerPort = available.Addr().(*net.TCPAddr).Port
	_ = available.Close()
	if _, err := a.SaveConfig(next); err != nil {
		t.Fatal(err)
	}
	newURL := a.GetRESTServerStatus().BaseURL
	if newURL == old.URL || !detectExistingRESTServer(newURL) {
		t.Fatal("新 REST listener 未就緒")
	}
	deadline := time.Now().Add(2 * time.Second)
	for detectExistingRESTServer(old.URL) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if detectExistingRESTServer(old.URL) {
		t.Fatal("舊 REST listener 未關閉")
	}
}

func TestBackgroundServiceFailurePreservesConcurrentPreferences(t *testing.T) {
	service, server := runningRESTApp(t)
	a := regressionApp(t, service.store)
	a.allowRESTAttach, a.restAttached, a.restServerURL = true, true, server.URL
	// 使用測試程序 PID 模擬已存在但未套用新設定的背景服務，避免啟動真實服務。
	if err := a.RegisterBackgroundService(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	available, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	previous := a.GetConfig()
	next := previous
	next.RESTServerPort = available.Addr().(*net.TCPAddr).Port
	next.Theme = "failed-change"
	_ = available.Close()
	done := make(chan error, 1)
	go func() { _, err := a.SaveConfig(next); done <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		saved, err := a.store.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if saved.RESTServerPort == next.RESTServerPort {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("設定未進入背景服務啟動階段")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := a.store.UpdateConfig(func(latest *model.Config) error {
		latest.Language = "later-language"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("背景服務未切換至新埠卻回報成功")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("背景服務啟動失敗未及時回傳")
	}
	saved, err := a.store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved.RESTServerPort != previous.RESTServerPort || saved.Theme != previous.Theme || saved.Language != "later-language" {
		t.Fatalf("設定還原遺失其他程序的修改: %+v", saved)
	}
	if a.GetConfig().RESTServerPort != previous.RESTServerPort || !detectExistingRESTServer(server.URL) {
		t.Fatal("背景服務啟動失敗後未保留原服務狀態")
	}
}
