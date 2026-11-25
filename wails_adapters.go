package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The Wails-backed implementations of the app's UI ports (EventEmitter, OpenDialogs). Each
// resolves the app context lazily: it only exists once Wails has started the app.

// wailsEvents emits events to the frontend through the Wails runtime.
type wailsEvents struct {
	ctx func() context.Context
}

func (e wailsEvents) Emit(name string, data ...any) {
	if ctx := e.ctx(); ctx != nil {
		runtime.EventsEmit(ctx, name, data...)
	}
}

// wailsOpenDialogs shows the native file and folder pickers.
type wailsOpenDialogs struct {
	ctx func() context.Context
}

func (d wailsOpenDialogs) PickFile() (string, error) {
	return runtime.OpenFileDialog(d.ctx(), runtime.OpenDialogOptions{Title: "Select File"})
}

func (d wailsOpenDialogs) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(d.ctx(), runtime.OpenDialogOptions{Title: "Select Folder"})
}
