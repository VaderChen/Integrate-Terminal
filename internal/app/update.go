package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/updater"
	"IntegTERM/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) CheckForUpdates() (model.UpdateCheckResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 20*time.Second)
	defer cancel()
	return updater.CheckLatest(ctx, version.UpdateVersion())
}

func (a *App) StartUpdate(expectedTag string, requestID string) (model.UpdateActionResult, error) {
	if !a.updateMu.TryLock() {
		return model.UpdateActionResult{}, fmt.Errorf("已有更新正在執行")
	}
	defer a.updateMu.Unlock()
	a.stateMu.RLock()
	eventContext := a.ctx
	a.stateMu.RUnlock()
	report := func(p updater.Progress) {
		p.RequestID = requestID
		if eventContext != nil {
			runtime.EventsEmit(eventContext, "update:progress", p)
		}
	}
	report(updater.Progress{Stage: "preparing"})
	ctx, cancel := context.WithTimeout(a.updateContext(), 15*time.Minute)
	defer cancel()
	action, err := updater.PrepareLatestWithProgress(ctx, version.UpdateVersion(), expectedTag, report)
	if err != nil {
		return model.UpdateActionResult{}, err
	}

	report(updater.Progress{Stage: "installing"})
	if action.Downloaded && strings.HasSuffix(strings.ToLower(action.Target), ".dmg") {
		if targetApp, ok := currentAppBundlePath(); ok {
			expectedProductVersion := action.Version
			expectedBundleVersion := action.Version
			if action.Build != "" {
				expectedProductVersion = strings.TrimSuffix(action.Version, "."+action.Build)
				expectedBundleVersion = expectedProductVersion + action.Build
			}
			if err := scheduleUpdateInstall(action.Target, targetApp, expectedProductVersion, expectedBundleVersion, os.Getpid()); err == nil {
				go func() {
					time.Sleep(250 * time.Millisecond)
					if eventContext != nil {
						runtime.Quit(eventContext)
					}
				}()
				return model.UpdateActionResult{Downloaded: true, InstallScheduled: true, Restarting: true}, nil
			} else {
				return model.UpdateActionResult{}, fmt.Errorf("cannot schedule update installation: %w", err)
			}
		}
	}
	commandName, arguments, err := openCommandForPath(action.Target)
	if err != nil {
		return model.UpdateActionResult{}, err
	}
	if err := startDetachedCommand(commandName, arguments...); err != nil {
		return model.UpdateActionResult{}, fmt.Errorf("cannot open update: %w", err)
	}
	return model.UpdateActionResult{Downloaded: action.Downloaded}, nil
}

func currentAppBundlePath() (string, bool) {
	executable, err := os.Executable()
	if err != nil {
		return "", false
	}
	path, err := filepath.Abs(executable)
	if err != nil {
		return "", false
	}
	for directory := filepath.Dir(path); directory != filepath.Dir(directory); directory = filepath.Dir(directory) {
		if strings.HasSuffix(strings.ToLower(directory), ".app") {
			return directory, true
		}
	}
	return "", false
}

func (a *App) updateContext() context.Context {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
