package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDocumentToolInvalidJSON(t *testing.T) {
	var p ParseDocumentTool
	s, err := p.Execute("{not json")
	if err != nil {
		t.Fatalf("Execute returns string errors to model: %v", err)
	}
	if !strings.Contains(s, "Error parsing arguments") {
		t.Fatalf("unexpected result: %q", s)
	}
}

func TestParseDocumentToolPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"path":   path,
		"offset": 1,
		"limit":  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var p ParseDocumentTool
	s, err := p.Execute(string(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "Parser: text") {
		t.Fatalf("expected parser header: %q", s)
	}
	if !strings.Contains(s, "1: alpha") {
		t.Fatalf("expected line content: %q", s)
	}
}
