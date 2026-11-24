package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"pedro/media"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveResult reports where an export was written. Cancelled is set when the
// person closed the save dialog without picking a location.
type SaveResult struct {
	Path      string `json:"path"`
	Cancelled bool   `json:"cancelled"`
}

// SaveDialog asks the person where to save a file. An empty path means they cancelled.
type SaveDialog interface {
	AskSavePath(defaultFilename string) (string, error)
}

// FileExporter writes content the chat shows (images, code, diagrams) to a
// location the person picks.
type FileExporter interface {
	// SaveImage exports an image by the src the chat displays it with: a URL of
	// a stored image (generated or pasted) or a base64 image data URL.
	SaveImage(src string) (SaveResult, error)
	// SaveBytes exports raw content under a suggested file name.
	SaveBytes(defaultFilename string, data []byte) (SaveResult, error)
}

// wailsSaveDialog shows the native save dialog of the running app.
type wailsSaveDialog struct {
	ctx func() context.Context
}

func (d wailsSaveDialog) AskSavePath(defaultFilename string) (string, error) {
	return runtime.SaveFileDialog(d.ctx(), runtime.SaveDialogOptions{DefaultFilename: defaultFilename})
}

// fileExporter resolves image URLs through the app's image stores.
type fileExporter struct {
	dialog SaveDialog
	stores []media.ImageStore
}

func newFileExporter(dialog SaveDialog, stores ...media.ImageStore) *fileExporter {
	return &fileExporter{dialog: dialog, stores: stores}
}

func (e *fileExporter) SaveImage(src string) (SaveResult, error) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "data:") {
		img, err := media.ImageFromDataURL(src)
		if err != nil {
			return SaveResult{}, err
		}
		return e.SaveBytes("image."+media.Extension(img.Format), img.Data)
	}

	path, ok := e.lookup(src)
	if !ok {
		return SaveResult{}, fmt.Errorf("not an image stored by Pedro: %s", src)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SaveResult{}, fmt.Errorf("read image: %w", err)
	}
	return e.SaveBytes(filepath.Base(path), data)
}

func (e *fileExporter) SaveBytes(defaultFilename string, data []byte) (SaveResult, error) {
	path, err := e.dialog.AskSavePath(defaultFilename)
	if err != nil {
		return SaveResult{}, fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return SaveResult{Cancelled: true}, nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("write file: %w", err)
	}
	return SaveResult{Path: path}, nil
}

// lookup maps an image URL to its file. The webview may hand over absolute
// URLs (http://wails.localhost/generated-images/x.png), so only the path counts.
func (e *fileExporter) lookup(src string) (string, bool) {
	u, err := url.Parse(src)
	if err != nil || u.Path == "" {
		return "", false
	}
	for _, s := range e.stores {
		if path, ok := s.Lookup(u.Path); ok {
			return path, true
		}
	}
	return "", false
}
