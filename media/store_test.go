package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirImageStoreSaveAndServe(t *testing.T) {
	store := NewDirImageStore(t.TempDir(), GeneratedImagesRoute)
	png := []byte("\x89PNG\r\n\x1a\nfake")

	stored, err := store.Save(Image{Data: png, Format: "png"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.URL, GeneratedImagesRoute) || !strings.HasSuffix(stored.URL, ".png") {
		t.Fatalf("unexpected URL %q", stored.URL)
	}
	if data, err := os.ReadFile(stored.Path); err != nil || string(data) != string(png) {
		t.Fatalf("file not written correctly: %v", err)
	}

	rec := httptest.NewRecorder()
	store.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, stored.URL, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != string(png) {
		t.Fatalf("serve: got %d %q", rec.Code, rec.Body.String())
	}
}

func TestDirImageStoreRejectsOtherPaths(t *testing.T) {
	store := NewDirImageStore(t.TempDir(), GeneratedImagesRoute)
	for _, target := range []string{
		"/",
		"/index.html",
		GeneratedImagesRoute,
		GeneratedImagesRoute + "missing.png",
		GeneratedImagesRoute + "..%2fsecret",
		GeneratedImagesRoute + "sub/dir.png",
		GeneratedImagesRoute + ".hidden",
	} {
		rec := httptest.NewRecorder()
		store.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", target, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	store.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, GeneratedImagesRoute+"x.png", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST: got %d, want 404", rec.Code)
	}
}

func TestDirImageStoreRejectsEmptyImage(t *testing.T) {
	if _, err := NewDirImageStore(t.TempDir(), GeneratedImagesRoute).Save(Image{}); err == nil {
		t.Fatal("expected error for empty image")
	}
}

func TestSwitch(t *testing.T) {
	s := NewSwitch()
	if s.Current().Images != nil || s.Current().Transcriber != nil {
		t.Fatal("new switch should be empty")
	}
	s.Set(Services{Images: &OpenAIImageGenerator{}})
	if s.Current().Images == nil {
		t.Fatal("services not set")
	}
	s.Set(Services{})
	if s.Current().Images != nil {
		t.Fatal("services not cleared")
	}
}

func TestDirImageStoreLookup(t *testing.T) {
	dir := t.TempDir()
	store := NewDirImageStore(dir, PastedImagesRoute)
	stored, err := store.Save(Image{Data: []byte("x"), Format: "jpeg"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.URL, PastedImagesRoute) || !strings.HasSuffix(stored.URL, ".jpg") {
		t.Fatalf("unexpected URL %q", stored.URL)
	}
	if path, ok := store.Lookup(stored.URL); !ok || path != stored.Path {
		t.Fatalf("lookup: got %q %v, want %q", path, ok, stored.Path)
	}
	for _, url := range []string{stored.Path, GeneratedImagesRoute + "x.png", PastedImagesRoute + "../x.png"} {
		if _, ok := store.Lookup(url); ok {
			t.Errorf("lookup %q should fail", url)
		}
	}
}

func TestImageFromDataURL(t *testing.T) {
	img, err := ImageFromDataURL("data:image/webp;base64,aGk=")
	if err != nil || string(img.Data) != "hi" || img.Format != "webp" {
		t.Fatalf("got %+v %v", img, err)
	}
	for _, bad := range []string{"", "hello", "data:text/plain;base64,aGk=", "data:image/png,raw", "data:image/png;base64,%%%"} {
		if _, err := ImageFromDataURL(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestDirImageStoreRemove(t *testing.T) {
	dir := t.TempDir()
	store := NewDirImageStore(dir, GeneratedImagesRoute)
	a, _ := store.Save(Image{Data: []byte("a"), Format: "png"})
	b, _ := store.Save(Image{Data: []byte("b"), Format: "png"})

	outside := filepath.Join(t.TempDir(), "keep.png")
	_ = os.WriteFile(outside, []byte("x"), 0o644)
	for _, p := range []string{outside, dir, filepath.Join(dir, ".hidden"), filepath.Join(dir, "sub", "x.png")} {
		if err := store.Remove(p); err == nil {
			t.Errorf("Remove(%q) should be refused", p)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("file outside the store was removed")
	}

	if err := store.Remove(a.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatal("image not removed")
	}
	if err := store.Remove(a.Path); err != nil {
		t.Fatalf("removing a missing image should succeed: %v", err)
	}

	if err := store.RemoveAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.Path); !os.IsNotExist(err) {
		t.Fatal("RemoveAll left an image")
	}
	if err := NewDirImageStore(filepath.Join(dir, "never-created"), GeneratedImagesRoute).RemoveAll(); err != nil {
		t.Fatalf("RemoveAll on a missing directory: %v", err)
	}
}
