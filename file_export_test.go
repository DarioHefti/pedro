package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"pedro/media"
)

// fakeSaveDialog answers every request with a fixed path and records the suggested name.
type fakeSaveDialog struct {
	path      string
	suggested string
}

func (d *fakeSaveDialog) AskSavePath(defaultFilename string) (string, error) {
	d.suggested = defaultFilename
	return d.path, nil
}

func newTestExporter(t *testing.T) (*fileExporter, *fakeSaveDialog, *media.DirImageStore) {
	t.Helper()
	store := media.NewDirImageStore(filepath.Join(t.TempDir(), "generated"), media.GeneratedImagesRoute)
	dialog := &fakeSaveDialog{path: filepath.Join(t.TempDir(), "out.bin")}
	return newFileExporter(dialog, store), dialog, store
}

func TestSaveImageCopiesStoredImage(t *testing.T) {
	exporter, dialog, store := newTestExporter(t)
	stored, err := store.Save(media.Image{Data: []byte("png-bytes"), Format: "png"})
	if err != nil {
		t.Fatal(err)
	}

	for _, src := range []string{stored.URL, "http://wails.localhost" + stored.URL, "wails://wails" + stored.URL} {
		res, err := exporter.SaveImage(src)
		if err != nil {
			t.Fatalf("SaveImage(%q): %v", src, err)
		}
		if res.Cancelled || res.Path != dialog.path {
			t.Fatalf("SaveImage(%q) = %+v, want path %s", src, res, dialog.path)
		}
		if dialog.suggested != filepath.Base(stored.Path) {
			t.Errorf("suggested name = %q, want %q", dialog.suggested, filepath.Base(stored.Path))
		}
		got, _ := os.ReadFile(dialog.path)
		if string(got) != "png-bytes" {
			t.Errorf("written content = %q", got)
		}
	}
}

func TestSaveImageDecodesDataURL(t *testing.T) {
	exporter, dialog, _ := newTestExporter(t)
	src := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("jpeg-bytes"))

	if _, err := exporter.SaveImage(src); err != nil {
		t.Fatal(err)
	}
	if dialog.suggested != "image.jpg" {
		t.Errorf("suggested name = %q, want image.jpg", dialog.suggested)
	}
	got, _ := os.ReadFile(dialog.path)
	if string(got) != "jpeg-bytes" {
		t.Errorf("written content = %q", got)
	}
}

func TestSaveImageRefusesUnknownImages(t *testing.T) {
	exporter, _, _ := newTestExporter(t)
	for _, src := range []string{"https://example.com/cat.png", "/generated-images/../secrets.txt", "/elsewhere/x.png", ""} {
		if _, err := exporter.SaveImage(src); err == nil {
			t.Errorf("SaveImage(%q) succeeded, want an error", src)
		}
	}
}

func TestSaveBytesReportsCancel(t *testing.T) {
	exporter, dialog, _ := newTestExporter(t)
	dialog.path = ""

	res, err := exporter.SaveBytes("diagram.svg", []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cancelled || res.Path != "" {
		t.Errorf("SaveBytes after cancel = %+v, want cancelled", res)
	}
}
