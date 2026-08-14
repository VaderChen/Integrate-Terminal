package main

import (
	"embed"
	"log"
	"os"

	"github.com/VaderChen/Integrate-Terminal/internal/app"
	"github.com/VaderChen/Integrate-Terminal/internal/crashlog"
	"github.com/VaderChen/Integrate-Terminal/internal/trayservice"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		crashlog.Init()
		defer crashlog.Recover("main.serve")
		trayservice.New().Run()
		return
	}

	runUI()
}

func runUI() {
	application := app.New()

	err := wails.Run(&options.App{
		Title:            "IntegTERM",
		Width:            1440,
		Height:           920,
		MinWidth:         660,
		MinHeight:        500,
		AssetServer:      &assetserver.Options{Assets: assets},
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true},
		BackgroundColour: &options.RGBA{R: 247, G: 243, B: 234, A: 1},
		OnStartup:        application.Startup,
		OnDomReady:       application.DomReady,
		OnShutdown:       application.Shutdown,
		Bind: []interface{}{
			application,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
