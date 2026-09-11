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
)

func (a *App) CheckForUpdates() (model.UpdateCheckResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 20*time.Second)
	defer cancel()
	return updater.CheckLatest(ctx, version.UpdateVersion())
}

func (a *App) StartUpdate(expectedTag string) (model.UpdateActionResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 15*time.Minute)
	defer cancel()
	action, err := updater.PrepareLatest(ctx, version.UpdateVersion(), expectedTag)
	if err != nil {
		return model.UpdateActionResult{}, err
	}

	if action.Downloaded && strings.HasSuffix(strings.ToLower(action.Target), ".dmg") {
		if targetApp, ok := currentAppBundlePath(); ok {
			if err := scheduleUpdateInstall(action.Target, targetApp, version.UpdateVersion(), os.Getpid()); err == nil {
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
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
