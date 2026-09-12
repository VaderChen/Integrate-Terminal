package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/updater"
	"github.com/VaderChen/Integrate-Terminal/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) CheckForUpdates() (model.UpdateCheckResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 20*time.Second)
	defer cancel()
	return updater.CheckLatest(ctx, version.UpdateVersion(), a.forceUpdateEnabled())
}

func (a *App) StartUpdate(expectedTag string) (model.UpdateActionResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 15*time.Minute)
	defer cancel()
	action, err := updater.PrepareLatest(ctx, version.UpdateVersion(), expectedTag, a.forceUpdateEnabled())
	if err != nil {
		return model.UpdateActionResult{}, err
	}

	if action.Downloaded && strings.HasSuffix(strings.ToLower(action.Target), ".dmg") {
		if targetApp, ok := currentAppBundlePath(); ok {
			expectedProductVersion := action.Version
			expectedBundleVersion := action.Version
			if action.Build != "" {
				expectedProductVersion = strings.TrimSuffix(action.Version, "."+action.Build)
				expectedBundleVersion = expectedProductVersion + action.Build
			}
			if err := scheduleUpdateInstall(action.Target, targetApp, expectedProductVersion, expectedBundleVersion, os.Getpid()); err == nil {
				_ = a.StopBackgroundService()
				a.ApproveQuit()
				go func() {
					time.Sleep(250 * time.Millisecond)
					if a.ctx != nil {
						runtime.Quit(a.ctx)
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

func (a *App) forceUpdateEnabled() bool {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.config.ForceUpdate
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
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
