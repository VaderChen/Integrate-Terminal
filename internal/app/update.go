package app

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/updater"
	"github.com/VaderChen/Integrate-Terminal/internal/version"
)

func (a *App) CheckForUpdates() (model.UpdateCheckResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 20*time.Second)
	defer cancel()
	return updater.CheckLatest(ctx, version.ProductVersion())
}

func (a *App) StartUpdate(expectedTag string) (model.UpdateActionResult, error) {
	ctx, cancel := context.WithTimeout(a.updateContext(), 15*time.Minute)
	defer cancel()
	action, err := updater.PrepareLatest(ctx, version.ProductVersion(), expectedTag)
	if err != nil {
		return model.UpdateActionResult{}, err
	}

	commandName, arguments, err := openCommandForPath(action.Target)
	if err != nil {
		return model.UpdateActionResult{}, err
	}
	command := exec.Command(commandName, arguments...)
	if err := command.Start(); err != nil {
		return model.UpdateActionResult{}, fmt.Errorf("cannot open update: %w", err)
	}
	go func() {
		_ = command.Wait()
	}()
	return model.UpdateActionResult{Downloaded: action.Downloaded}, nil
}

func (a *App) updateContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
