package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()
	updater := NewUpdater()

	appOptions := &options.App{
		Title:     "Pedro",
		Width:     840,
		Height:    640,
		MinWidth:  808,
		MinHeight: 608,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.assetHandler(),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			updater.startup(ctx)
		},
		OnShutdown: app.shutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{
			app,
			updater,
		},
	}
	// Per-OS tweaks (Linux: Wayland app-id, tiling-friendly sizing, GPU workarounds).
	applyPlatformOptions(appOptions)

	if err := wails.Run(appOptions); err != nil {
		println("Error:", err.Error())
	}
}
