package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeImageDataURLsFromFileRefs_IncludesPNGPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "one.png")
	raw, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	atts, err := json.Marshal([]map[string]string{
		{"type": "file-ref", "content": p, "name": "one.png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := mergeImageDataURLsFromFileRefs(nil, string(atts), "")
	if len(out) != 1 {
		t.Fatalf("got %d data URLs, want 1", len(out))
	}
	if !strings.HasPrefix(out[0], "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %q", out[0])
	}
}

func TestMergeImageDataURLsFromFileRefs_SkipsNonImage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	atts, err := json.Marshal([]map[string]string{
		{"type": "file-ref", "content": p, "name": "note.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := mergeImageDataURLsFromFileRefs(nil, string(atts), "")
	if len(out) != 0 {
		t.Fatalf("got %d, want 0", len(out))
	}
}

func TestMergeImageDataURLsFromFileRefs_PathMarkerFallback(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fallback.png")
	raw, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	content := "Please analyze this image\n[Path: " + p + "]"
	out := mergeImageDataURLsFromFileRefs(nil, "", content)
	if len(out) != 1 {
		t.Fatalf("got %d data URLs, want 1", len(out))
	}
	if !strings.HasPrefix(out[0], "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %q", out[0])
	}
}

func TestMergeImageDataURLsFromFileRefs_PathMarkerCaseAndQuotes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "quoted.png")
	raw, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	content := "Check this\n[path: \"" + p + "\"]"
	out := mergeImageDataURLsFromFileRefs(nil, "", content)
	if len(out) != 1 {
		t.Fatalf("got %d data URLs, want 1", len(out))
	}
	if !strings.HasPrefix(out[0], "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %q", out[0])
	}
}
