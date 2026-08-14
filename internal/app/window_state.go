package app

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) captureWindowState() {
	if a.ctx == nil {
		return
	}

	if !a.config.RememberWindowPosition || !wailsruntime.WindowIsNormal(a.ctx) {
		return
	}

	width, height := wailsruntime.WindowGetSize(a.ctx)
	if width > 0 {
		a.config.WindowWidth = width
	}
	if height > 0 {
		a.config.WindowHeight = height
	}

	x, y := wailsruntime.WindowGetPosition(a.ctx)
	a.config.WindowX = x
	a.config.WindowY = y
}

func (a *App) applyInitialWindowPlacement() {
	if a.ctx == nil {
		return
	}

	if !a.config.RememberWindowPosition {
		wailsruntime.WindowCenter(a.ctx)
		return
	}

	width := a.config.WindowWidth
	height := a.config.WindowHeight
	if width <= 0 {
		width = 1440
	}
	if height <= 0 {
		height = 920
	}
	wailsruntime.WindowSetSize(a.ctx, width, height)

	if !a.isWindowPositionPlausible(a.config.WindowX, a.config.WindowY, width, height) {
		wailsruntime.WindowCenter(a.ctx)
		return
	}

	wailsruntime.WindowSetPosition(a.ctx, a.config.WindowX, a.config.WindowY)
}

func (a *App) isWindowPositionPlausible(x int, y int, width int, height int) bool {
	screens, err := wailsruntime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return x != 0 || y != 0
	}

	totalWidth := 0
	maxHeight := 0
	for _, screen := range screens {
		screenWidth := screen.Size.Width
		screenHeight := screen.Size.Height
		if screenWidth <= 0 {
			screenWidth = screen.Width
		}
		if screenHeight <= 0 {
			screenHeight = screen.Height
		}
		totalWidth += screenWidth
		if screenHeight > maxHeight {
			maxHeight = screenHeight
		}
	}

	if totalWidth <= 0 || maxHeight <= 0 {
		return x != 0 || y != 0
	}

	minX := -totalWidth
	maxX := totalWidth
	minY := -maxHeight
	maxY := maxHeight

	return x+width > minX && x < maxX && y+height > minY && y < maxY
}
