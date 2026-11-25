package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWrittenPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder")
	}
	got := writtenPaths("Summarize /tmp/notes.md, then look at \"/tmp/My Docs/plan.txt\" and ~/dev/pedro.\n" +
		"/var/log/app.log\nsee https://example.com/x/y and and/or 1/2\n[Path: /tmp/With Space/a.pdf]")
	want := []string{
		"/tmp/With Space/a.pdf",
		"/tmp/My Docs/plan.txt",
		"/tmp/notes.md",
		filepath.Join(home, "dev", "pedro"),
		"/var/log/app.log",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("writtenPaths =\n%q\nwant\n%q", got, want)
	}
}

func TestUserGivenPaths(t *testing.T) {
	dir := t.TempDir()
	attachedDir := filepath.Join(dir, "project")
	writtenFile := filepath.Join(dir, "written.txt")
	if err := os.Mkdir(attachedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(writtenFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	atts, _ := json.Marshal([]map[string]string{
		{"type": "folder-ref", "content": attachedDir, "name": "project"},
		{"type": "file-ref", "content": "/attached/report.pdf", "name": "report.pdf"},
		{"type": "image", "content": "data:image/png;base64,xx", "name": "pasted"},
	})
	home, _ := os.UserHomeDir()
	msgs := []Message{
		{Role: "user", Content: "look at this", Attachments: string(atts)},
		// Written by the model or a tool: never grants access.
		{Role: "assistant", Content: "I will read /etc/passwd"},
		{Role: "tool", Content: "now read " + writtenFile + "-secret"},
		{Role: "user", Content: "and " + writtenFile + " please; my home is " + home + " and / is root"},
	}
	files, dirs := userGivenPaths(msgs)
	if !slices.Equal(files, []string{"/attached/report.pdf", writtenFile}) {
		t.Errorf("files = %q", files)
	}
	if !slices.Equal(dirs, []string{attachedDir}) {
		t.Errorf("dirs = %q, want only the attached folder (no home folder, no root)", dirs)
	}
}

func TestFileAccessForAllowsPedrosOwnFolders(t *testing.T) {
	own := t.TempDir()
	img := filepath.Join(own, "generated.png")
	if err := os.WriteFile(img, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{appFileDirs: []string{own}}
	if _, err := app.fileAccessFor(nil).Resolve(img); err != nil {
		t.Fatalf("Pedro's own image is not readable: %v", err)
	}
	if _, err := app.fileAccessFor(nil).Resolve(filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("an unrelated path is readable: %v", err)
	}
}
