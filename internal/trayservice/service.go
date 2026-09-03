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
	serviceLock    *app.BackgroundServiceLock
	startErr       error
	mu             sync.Mutex
}

func New() (*Service, error) {
	crashlog.Init()
	executablePath, err := os.Executable()
	if err != nil {
		executablePath = ""
	}
	serviceApp := app.New()
	serviceLock, err := serviceApp.AcquireBackgroundServiceLock()
	if err != nil {
		return nil, err
	}
	serviceApp.ServiceStartup()
	if err := serviceApp.RegisterBackgroundService(os.Getpid()); err != nil {
		_ = serviceLock.Release()
		return nil, err
	}

	return &Service{
		executablePath: executablePath,
		app:            serviceApp,
		serviceLock:    serviceLock,
		startErr:       nil,
	}, nil
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

	statusHeader := systray.AddMenuItem(messages.statusSection, messages.statusSectionHint)
	statusHeader.Disable()

	status := s.restStatus()
	localStatusItem := systray.AddMenuItem(labelValue(messages.localService, messages.runningValue), "本機 VFS 服務狀態")
	localStatusItem.Disable()
	remoteStatusItem := systray.AddMenuItem(labelValue(messages.remoteService, serviceStatusValue(messages, status)), "遠端 HTTP MCP 服務狀態")
	remoteStatusItem.Disable()

	connectionLabel := labelValue(messages.backgroundConnections, itoa(s.currentBackgroundCount()))
	connectionItem := systray.AddMenuItem(connectionLabel, messages.backgroundConnectionsHint)
	connectionItem.Disable()

	versionItem := systray.AddMenuItem(labelValue(messages.versionNumber, version.Current()), messages.versionHint)
	versionItem.Disable()

	localEndpointURI := "integterm-vfs://workspace/mcp"
	localEndpointItem := systray.AddMenuItem("本機服務："+localEndpointURI, "點擊複製本機 VFS Resource URI；本機 Agent 請使用 mcp stdio 指令")
	remoteEndpointItem := systray.AddMenuItem("遠端服務："+status.MCPURL, "點擊複製遠端 HTTP MCP URL")
	if !status.Running || status.MCPURL == "" {
		remoteEndpointItem.Hide()
	}

	systray.AddSeparator()
	actionHeader := systray.AddMenuItem(messages.actionSection, messages.actionSectionHint)
	actionHeader.Disable()

	openUIItem := systray.AddMenuItem(messages.openMainWindow, messages.openMainWindowHint)
	clearBackgroundItem := systray.AddMenuItem(messages.clearBackground, messages.clearBackgroundHint)
	quitItem := systray.AddMenuItem(messages.quitBackground, messages.quitBackgroundHint)

	go s.refreshLoop(localStatusItem, remoteStatusItem, connectionItem, versionItem, remoteEndpointItem)

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
				// Tray 的結束動作直接停止背景服務，不經過 UI 的 Cmd+Q 確認流程。
				s.app.ServiceShutdown()
				s.stopRunningUI()
				systray.Quit()
				return
			case <-localEndpointItem.ClickedCh:
				_ = copyTrayText(localEndpointURI)
			case <-remoteEndpointItem.ClickedCh:
				if status := s.restStatus(); status.Running {
					_ = copyTrayText(status.MCPURL)
				}
			}
		}
	}()
}

func (s *Service) stopRunningUI() {
	pids := make(map[int]struct{})
	if s.app != nil {
		for _, pid := range s.app.ForegroundUIProcessIDs() {
			pids[pid] = struct{}{}
		}
	}

	// Keep a command-line fallback for UI instances started by an older build
	// before the foreground PID registry was introduced.
	patterns := make([]string, 0, 2)
	if s.executablePath != "" {
		patterns = append(patterns, s.executablePath)
	}
	if stdruntime.GOOS == "darwin" {
		patterns = append(patterns, "/Contents/MacOS/IntegTERM")
	}
	for _, pattern := range patterns {
		output, err := exec.Command("pgrep", "-f", pattern).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil && pid > 0 {
				pids[pid] = struct{}{}
			}
		}
	}

	for pid := range pids {
		if pid <= 0 || pid == os.Getpid() {
			continue
		}
		s.terminateUIProcess(pid)
	}
}

func (s *Service) terminateUIProcess(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	// Ask the UI to exit first so Wails can persist its state. If the process
	// does not leave promptly, force it down so the Dock icon cannot remain.
	if stdruntime.GOOS == "windows" {
		_ = process.Kill()
		return
	}
	if err := exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run(); err != nil {
		_ = process.Kill()
		return
	}
	for range 20 {
		if exec.Command("kill", "-0", strconv.Itoa(pid)).Run() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = process.Kill()
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
	if s.serviceLock != nil {
		_ = s.serviceLock.Release()
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

func (s *Service) refreshLoop(localStatusItem *systray.MenuItem, remoteStatusItem *systray.MenuItem, connectionItem *systray.MenuItem, versionItem *systray.MenuItem, remoteEndpointItem *systray.MenuItem) {
	defer crashlog.Recover("trayservice.refreshLoop")
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	lastIdleSweep := time.Time{}
	lastTrayTitle := ""
	lastTrayTooltip := ""
	lastLocalStatusTitle := ""
	lastRemoteStatusTitle := ""
	lastConnectionTitle := ""
	lastVersionTitle := ""
	lastAddressTitle := ""
	status := s.restStatus()
	remoteEndpointVisible := status.Running && status.MCPURL != ""

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

		localStatusText := labelValue(messages.localService, messages.runningValue)
		if localStatusText != lastLocalStatusTitle {
			localStatusItem.SetTitle(localStatusText)
			lastLocalStatusTitle = localStatusText
		}
		remoteStatusText := labelValue(messages.remoteService, serviceStatusValue(messages, status))
		if remoteStatusText != lastRemoteStatusTitle {
			remoteStatusItem.SetTitle(remoteStatusText)
			lastRemoteStatusTitle = remoteStatusText
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

		addressLabel := "遠端服務：" + status.MCPURL
		if addressLabel != lastAddressTitle {
			remoteEndpointItem.SetTitle(addressLabel)
			lastAddressTitle = addressLabel
		}
		nextRemoteEndpointVisible := status.Running && status.MCPURL != ""
		if nextRemoteEndpointVisible != remoteEndpointVisible {
			if nextRemoteEndpointVisible {
				remoteEndpointItem.Show()
			} else {
				remoteEndpointItem.Hide()
			}
			remoteEndpointVisible = nextRemoteEndpointVisible
		}
	}
}

func copyTrayText(value string) error {
	command := exec.Command("pbcopy")
	command.Stdin = strings.NewReader(value)
	return command.Run()
}

func serviceStatusValue(messages trayMessages, status model.RESTServerStatus) string {
	if status.Running {
		return messages.runningValue
	}
	return messages.stoppedValue
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
