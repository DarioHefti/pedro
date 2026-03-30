package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create the app service
	appService := NewApp()

	// Create application
	app := application.New(application.Options{
		Name:        "Pedro",
		Description: "Pedro AI Chat",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			if appService.store != nil {
				appService.store.Close()
			}
		},
	})

	// Store app reference in service for events
	appService.app = app

	// Initialize LLM after app is created
	appService.initLLM()

	// Create the main window
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Pedro",
		Width:     840,
		Height:    640,
		MinWidth:  808,
		MinHeight: 608,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
		EnableFileDrop:   true,
	})

	// Run the application
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
