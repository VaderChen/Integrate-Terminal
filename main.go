package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/VaderChen/Integrate-Terminal/internal/app"
	"github.com/VaderChen/Integrate-Terminal/internal/crashlog"
	"github.com/VaderChen/Integrate-Terminal/internal/trayservice"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		crashlog.Init()
		defer crashlog.Recover("main.serve")
		service, err := trayservice.New()
		if errors.Is(err, app.ErrBackgroundServiceAlreadyRunning) {
			log.Printf("背景服務已在執行中")
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		service.Run()
		return
	}

	runUI(hasArgument("--multi-instance"))
}

func runUI(allowMultipleInstances bool) {
	application := app.New()
	uiPID := os.Getpid()
	if err := application.RegisterForegroundUI(uiPID); err != nil {
		log.Printf("register foreground UI failed: %v", err)
	}
	defer func() {
		if err := application.UnregisterForegroundUI(uiPID); err != nil {
			log.Printf("unregister foreground UI failed: %v", err)
		}
	}()

	var contextMu sync.RWMutex
	var uiContext context.Context
	secondInstancePending := false

	activateWindow := func(ctx context.Context) {
		if ctx == nil {
			contextMu.Lock()
			secondInstancePending = true
			contextMu.Unlock()
			return
		}
		runtime.WindowShow(ctx)
		runtime.WindowUnminimise(ctx)
	}

	onStartup := func(ctx context.Context) {
		application.Startup(ctx)
		contextMu.Lock()
		uiContext = ctx
		pending := secondInstancePending
		secondInstancePending = false
		contextMu.Unlock()
		if pending {
			activateWindow(ctx)
		}

	}

	onBeforeClose := func(ctx context.Context) bool {
		if application.ConsumeQuitApprovalForUI() {
			return false
		}
		// 點擊視窗關閉鈕時直接結束 UI，讓 Dock 圖示同步消失。
		// Cmd+Q 由前端快捷鍵攔截並顯示三選項確認視窗。
		return false
	}

	var singleInstanceLock *options.SingleInstanceLock
	if !allowMultipleInstances {
		singleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "com.vader.integterm.ui",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				contextMu.RLock()
				ctx := uiContext
				contextMu.RUnlock()
				activateWindow(ctx)
			},
		}
	}

	err := wails.Run(&options.App{
		Title:              "IntegTERM",
		Width:              1440,
		Height:             920,
		MinWidth:           660,
		MinHeight:          500,
		AssetServer:        &assetserver.Options{Assets: assets},
		DragAndDrop:        &options.DragAndDrop{EnableFileDrop: true},
		BackgroundColour:   &options.RGBA{R: 247, G: 243, B: 234, A: 1},
		OnStartup:          onStartup,
		OnDomReady:         application.DomReady,
		OnShutdown:         application.Shutdown,
		OnBeforeClose:      onBeforeClose,
		SingleInstanceLock: singleInstanceLock,
		Bind: []interface{}{
			application,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func hasArgument(argument string) bool {
	for _, value := range os.Args[1:] {
		if value == argument {
			return true
		}
	}
	return false
}
