package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"IntegTERM/internal/model"
)

func (a *App) ReloadConfig() model.Config {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	cfg, err := a.store.LoadConfig()
	if err == nil {
		a.config = cfg
		a.config.ProUnlock = a.verifiedProUnlock
		a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
	}
	return cloneConfig(a.config)
}

func (a *App) ConnectionCounts() (background int, foreground int) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	for _, tab := range a.tabs {
		if !tab.Connected {
			continue
		}
		if tab.Hidden {
			background++
			continue
		}
		foreground++
	}
	return background, foreground
}

func (a *App) ensureBackgroundService(config model.Config) error {
	if a.backgroundServiceRunning() {
		if config.RESTServerEnabled {
			baseURL := fmt.Sprintf("http://127.0.0.1:%d", sanitizeRESTServerPort(config.RESTServerPort))
			for range 15 {
				if detectExistingRESTServer(baseURL) {
					return nil
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
		return nil
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", sanitizeRESTServerPort(config.RESTServerPort))
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(executablePath, "serve")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	if !config.RESTServerEnabled {
		for range 15 {
			if a.backgroundServiceRunning() {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("background service did not become ready")
	}

	for range 15 {
		if detectExistingRESTServer(baseURL) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("background service did not become ready at %s", baseURL)
}

func shouldRunBackgroundService(config model.Config) bool {
	return config.ShowTrayIcon || config.RESTServerEnabled
}

func (a *App) backgroundServicePIDPath() string {
	return filepath.Join(a.store.BaseDir(), "background-service.pid")
}

func (a *App) backgroundServiceRunning() bool {
	data, err := os.ReadFile(a.backgroundServicePIDPath())
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(a.backgroundServicePIDPath())
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(a.backgroundServicePIDPath())
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(a.backgroundServicePIDPath())
		return false
	}
	return true
}

func (a *App) RegisterBackgroundService(pid int) error {
	return os.WriteFile(a.backgroundServicePIDPath(), []byte(strconv.Itoa(pid)), 0o644)
}

func (a *App) UnregisterBackgroundService() error {
	return os.Remove(a.backgroundServicePIDPath())
}

func (a *App) syncAttachedRESTState(config model.Config, allowAttach bool) {
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()

	a.restServer = nil
	a.restAttached = false
	a.restServerURL = ""

	if !allowAttach || !config.RESTServerEnabled {
		return
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", sanitizeRESTServerPort(config.RESTServerPort))
	if detectExistingRESTServer(baseURL) {
		a.restAttached = true
		a.restServerURL = baseURL
	}
}

func (a *App) ReloadRuntimeConfig() (model.Config, error) {
	// Never hold stateMu while Shutdown drains requests that also need it.
	a.runtimeConfigMu.Lock()
	defer a.runtimeConfigMu.Unlock()
	cfg, err := a.store.LoadConfig()
	if err != nil {
		return a.GetConfig(), err
	}
	a.stateMu.Lock()
	cfg.ProUnlock = a.verifiedProUnlock
	cfg.RESTServerPort = sanitizeRESTServerPort(cfg.RESTServerPort)
	a.config = cfg
	allowAttach := a.allowRESTAttach
	a.stateMu.Unlock()
	if allowAttach {
		if shouldRunBackgroundService(cfg) {
			if err := a.ensureBackgroundService(cfg); err != nil {
				return cloneConfig(cfg), err
			}
		}
		a.syncAttachedRESTState(cfg, allowAttach)
		return cloneConfig(cfg), nil
	}
	return cloneConfig(cfg), a.applyRESTServerConfig(cfg, allowAttach)
}
