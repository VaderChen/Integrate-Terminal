package trayservice

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/app"
	"github.com/VaderChen/Integrate-Terminal/internal/crashlog"
	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/trayicon"
	"github.com/VaderChen/Integrate-Terminal/internal/version"

	"github.com/getlantern/systray"
)

type Service struct {
	executablePath string
	app            *app.App
	startErr       error
	mu             sync.Mutex
}

func New() *Service {
	crashlog.Init()
	executablePath, err := os.Executable()
	if err != nil {
		executablePath = ""
	}
	serviceApp := app.New()
	serviceApp.ServiceStartup()
	_ = serviceApp.RegisterBackgroundService(os.Getpid())

	return &Service{
		executablePath: executablePath,
		app:            serviceApp,
		startErr:       nil,
	}
}

func (s *Service) Run() {
	systray.SetReopenHandler(func() {
		log.Printf("macOS reopen event received")
		if err := s.openUI(); err != nil {
			log.Printf("open ui from reopen event failed: %v", err)
		}
	})
	systray.Run(s.onReady, s.onExit)
}

func (s *Service) onReady() {
	messages := s.messages()

	systray.SetTemplateIcon(trayicon.MainPNG, trayicon.MainPNG)
	systray.SetTitle(s.backgroundConnectionTitle())
	systray.SetTooltip(messages.tooltip)

	statusText := messages.statusFailed
	statusTooltip := messages.serviceStartupFailed
	if status := s.restStatus(); status.Running {
		statusText = labelValue(messages.serviceStatus, messages.runningValue)
		statusTooltip = status.BaseURL
	} else if s.startErr == nil {
		statusText = labelValue(messages.serviceStatus, messages.stoppedValue)
		statusTooltip = messages.serviceStoppedHint
	}

	statusHeader := systray.AddMenuItem(messages.statusSection, messages.statusSectionHint)
	statusHeader.Disable()

	statusItem := systray.AddMenuItem(statusText, statusTooltip)
	statusItem.Disable()

	connectionLabel := labelValue(messages.backgroundConnections, itoa(s.currentBackgroundCount()))
	connectionItem := systray.AddMenuItem(connectionLabel, messages.backgroundConnectionsHint)
	connectionItem.Disable()

	status := s.restStatus()
	addressLabel := labelValue(messages.backendService, messages.backendUnavailable)
	if status.BaseURL != "" {
		addressLabel = labelValue(messages.backendService, status.BaseURL)
	} else if status.Port > 0 {
		addressLabel = labelValue(messages.backendService, "127.0.0.1:"+itoa(status.Port))
	}
	addressItem := systray.AddMenuItem(addressLabel, messages.backendServiceHint)
	addressItem.Disable()

	versionItem := systray.AddMenuItem(labelValue(messages.versionNumber, version.Current()), messages.versionHint)
	versionItem.Disable()

	systray.AddSeparator()

	actionHeader := systray.AddMenuItem(messages.actionSection, messages.actionSectionHint)
	actionHeader.Disable()

	openUIItem := systray.AddMenuItem(messages.openMainWindow, messages.openMainWindowHint)
	clearBackgroundItem := systray.AddMenuItem(messages.clearBackground, messages.clearBackgroundHint)
	quitItem := systray.AddMenuItem(messages.quitBackground, messages.quitBackgroundHint)

	go s.refreshLoop(statusItem, connectionItem, versionItem, addressItem)

	go func() {
		defer crashlog.Recover("trayservice.actionLoop")
		for {
			select {
			case <-openUIItem.ClickedCh:
				log.Printf("tray action clicked: open main window")
				if err := s.openUI(); err != nil {
					log.Printf("open ui failed: %v", err)
				}
			case <-clearBackgroundItem.ClickedCh:
				log.Printf("tray action clicked: clear background connections")
				s.clearBackgroundConnections()
			case <-quitItem.ClickedCh:
				log.Printf("tray action clicked: quit background service")
				systray.Quit()
				return
			}
		}
	}()
}

func (s *Service) openUI() error {
	if s.executablePath == "" {
		return os.ErrNotExist
	}

	var launchErrs []error
	for _, attempt := range s.uiLaunchCommands() {
		log.Printf("launch ui via: %q %q", attempt.Path, attempt.Args)
		if err := attempt.Start(); err != nil {
			launchErrs = append(launchErrs, err)
			continue
		}
		pid := 0
		if attempt.Process != nil {
			pid = attempt.Process.Pid
		}
		if err := attempt.Process.Release(); err != nil {
			log.Printf("release ui launch process failed: %v", err)
		}
		if err := s.activateUIWithRetry(pid); err != nil {
			log.Printf("activate ui failed after launch: %v", err)
		}
		return nil
	}
	if len(launchErrs) == 0 {
		return fmt.Errorf("no ui launch command available")
	}
	return errors.Join(launchErrs...)
}

func (s *Service) uiLaunchCommands() []*exec.Cmd {
	var commands []*exec.Cmd
	if strings.Contains(s.executablePath, "/go-build") {
		wd, err := os.Getwd()
		if err == nil && wd != "" {
			commands = append(commands, detachedCommand(wd, "go", "run", "."))
		}
	} else {
		commands = append(commands, detachedCommand("", s.executablePath))
	}

	if bundlePath := appBundlePath(s.executablePath); bundlePath != "" {
		commands = append(commands,
			detachedCommand("", "open", "-na", bundlePath),
			detachedCommand("", "open", "-a", bundlePath),
		)
	}

	return commands
}

func (s *Service) activateUI(pid int) error {
	if stdruntime.GOOS != "darwin" {
		return nil
	}
	if pid <= 0 {
		return fmt.Errorf("missing gui pid")
	}

	cmd := detachedCommand("", "osascript",
		"-e", fmt.Sprintf(`tell application "System Events" to set frontmost of first process whose unix id is %d to true`, pid),
	)
	return cmd.Run()
}

func (s *Service) activateUIWithRetry(pid int) error {
	var lastErr error
	for range 12 {
		if err := s.activateUI(pid); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr == nil {
		return fmt.Errorf("activate ui failed")
	}
	return lastErr
}

func (s *Service) onExit() {
	if s.app != nil {
		_ = s.app.UnregisterBackgroundService()
		s.app.ServiceShutdown()
	}
	log.Println("tray service exited")
}

func (s *Service) restStatus() model.RESTServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return model.RESTServerStatus{}
	}
	return s.app.GetRESTServerStatus()
}

func (s *Service) backgroundConnectionTitle() string {
	background, _ := s.connectionCounts()
	return fmt.Sprintf("ACT\n%d", background)
}

func (s *Service) currentBackgroundCount() int {
	background, _ := s.connectionCounts()
	return background
}

func (s *Service) connectionCounts() (background int, foreground int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return 0, 0
	}
	return s.app.ConnectionCounts()
}

func (s *Service) refreshLoop(statusItem *systray.MenuItem, connectionItem *systray.MenuItem, versionItem *systray.MenuItem, addressItem *systray.MenuItem) {
	defer crashlog.Recover("trayservice.refreshLoop")
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	lastIdleSweep := time.Time{}
	lastTrayTitle := ""
	lastTrayTooltip := ""
	lastStatusTitle := ""
	lastConnectionTitle := ""
	lastVersionTitle := ""
	lastAddressTitle := ""

	for range ticker.C {
		messages := s.messages()
		if cfg, err := s.syncRuntimeConfig(); err == nil {
			if !cfg.ShowTrayIcon && !cfg.RESTServerEnabled {
				systray.Quit()
				return
			}
		}
		if lastIdleSweep.IsZero() || time.Since(lastIdleSweep) >= time.Minute {
			s.closeIdleBackgroundConnections(15 * time.Minute)
			lastIdleSweep = time.Now()
		}
		status := s.restStatus()
		background, _ := s.connectionCounts()

		nextTrayTitle := fmt.Sprintf("ACT\n%d", background)
		if nextTrayTitle != lastTrayTitle {
			systray.SetTitle(nextTrayTitle)
			lastTrayTitle = nextTrayTitle
		}
		if messages.tooltip != lastTrayTooltip {
			systray.SetTooltip(messages.tooltip)
			lastTrayTooltip = messages.tooltip
		}

		statusText := messages.statusFailed
		if status.Running {
			statusText = labelValue(messages.serviceStatus, messages.runningValue)
		} else if s.startErr == nil {
			statusText = labelValue(messages.serviceStatus, messages.stoppedValue)
		}
		if statusText != lastStatusTitle {
			statusItem.SetTitle(statusText)
			lastStatusTitle = statusText
		}

		nextConnectionTitle := labelValue(messages.backgroundConnections, itoa(background))
		if nextConnectionTitle != lastConnectionTitle {
			connectionItem.SetTitle(nextConnectionTitle)
			lastConnectionTitle = nextConnectionTitle
		}

		nextVersionTitle := labelValue(messages.versionNumber, version.Current())
		if nextVersionTitle != lastVersionTitle {
			versionItem.SetTitle(nextVersionTitle)
			lastVersionTitle = nextVersionTitle
		}

		addressLabel := labelValue(messages.backendService, messages.backendUnavailable)
		if status.BaseURL != "" {
			addressLabel = labelValue(messages.backendService, status.BaseURL)
		} else if status.Port > 0 {
			addressLabel = labelValue(messages.backendService, "127.0.0.1:"+itoa(status.Port))
		}
		if addressLabel != lastAddressTitle {
			addressItem.SetTitle(addressLabel)
			lastAddressTitle = addressLabel
		}
	}
}

func (s *Service) closeIdleBackgroundConnections(idleLimit time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return
	}
	closedCount := s.app.CloseIdleHiddenConnections(idleLimit)
	if closedCount > 0 {
		log.Printf("closed %d idle background connection(s)", closedCount)
	}
}

func (s *Service) clearBackgroundConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return
	}
	closedCount := s.app.ClearBackgroundConnections()
	if closedCount > 0 {
		log.Printf("cleared %d background connection(s)", closedCount)
	}
}

func (s *Service) syncRuntimeConfig() (model.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return model.Config{}, nil
	}
	cfg, err := s.app.ReloadRuntimeConfig()
	if err != nil {
		s.startErr = err
		return cfg, err
	}
	s.startErr = nil
	return cfg, nil
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func labelValue(label string, value string) string {
	return label + "：" + value
}

func appBundlePath(executablePath string) string {
	marker := ".app/Contents/MacOS/"
	index := strings.Index(executablePath, marker)
	if index < 0 {
		return ""
	}
	return filepath.Clean(executablePath[:index+len(".app")])
}

func detachedCommand(workdir string, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdout = nullFile
		cmd.Stderr = nullFile
		cmd.Stdin = nullFile
	}
	return cmd
}
