package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GrepTool tests
// ---------------------------------------------------------------------------

func TestGrepCaseInsensitiveFlagNotCorrupted(t *testing.T) {
	// Before the fix, flags was initialized to "0", prepending a literal "0"
	// to the pattern when case_insensitive=false. Verify the plain search
	// doesn't accidentally inject "0" into the regex.
	dir := t.TempDir()
	f := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(f, []byte("hello world\n0hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewGrepTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":          "^hello",
		"path":             f,
		"case_insensitive": false,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	// Should find the "hello world" line but NOT "0hello".
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected to match 'hello world', got: %q", out)
	}
	if strings.Contains(out, "0hello") {
		t.Errorf("unexpectedly matched '0hello'; the flags-init bug may still be present: %q", out)
	}
}

func TestGrepCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("Hello\nworld\nHELLO\n"), 0o644)

	g := NewGrepTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":          "hello",
		"path":             f,
		"case_insensitive": true,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Found 2") {
		t.Errorf("expected 2 case-insensitive matches, got: %q", out)
	}
}

func TestGrepMaxMatches(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.txt")
	var sb strings.Builder
	for i := 0; i < 300; i++ {
		sb.WriteString("match line\n")
	}
	os.WriteFile(f, []byte(sb.String()), 0o644)

	g := NewGrepTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":     "match",
		"path":        f,
		"max_matches": 50,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Found 50") {
		t.Errorf("expected truncation at 50, got: %q", out)
	}
	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation notice, got: %q", out)
	}
}

func TestGrepBraceExpansion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.ts"), []byte("found it\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.tsx"), []byte("found it\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("found it\n"), 0o644)

	g := NewGrepTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":      "found",
		"path":         dir,
		"file_pattern": "*.{ts,tsx}",
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.ts") || !strings.Contains(out, "b.tsx") {
		t.Errorf("expected .ts and .tsx matches, got: %q", out)
	}
	if strings.Contains(out, "c.go") {
		t.Errorf("should not match .go file, got: %q", out)
	}
}

func TestExpandBraces(t *testing.T) {
	cases := []struct {
		pattern string
		want    []string
	}{
		{"*.go", []string{"*.go"}},
		{"*.{ts,tsx}", []string{"*.ts", "*.tsx"}},
		{"*.{go,ts,tsx}", []string{"*.go", "*.ts", "*.tsx"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := expandBraces(tc.pattern)
		if len(got) != len(tc.want) {
			t.Errorf("expandBraces(%q) = %v, want %v", tc.pattern, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("expandBraces(%q)[%d] = %q, want %q", tc.pattern, i, got[i], tc.want[i])
			}
		}
	}
}

func TestGrepSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.Mkdir(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "config"), []byte("needle\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "src.go"), []byte("needle\n"), 0o644)

	g := NewGrepTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern": "needle",
		"path":    dir,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ".git") {
		t.Errorf("should skip .git directory, got: %q", out)
	}
	if !strings.Contains(out, "src.go") {
		t.Errorf("expected to find match in src.go, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// GlobTool tests
// ---------------------------------------------------------------------------

func TestGlobNoDoublestar(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte{}, 0o644)

	g := NewGlobTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":  "*.go",
		"base_dir": dir,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Found 2") {
		t.Errorf("expected 2 .go matches, got: %q", out)
	}
}

func TestGlobDoublestar(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(sub, "b.go"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(sub, "c.ts"), []byte{}, 0o644)

	g := NewGlobTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":  "**/*.go",
		"base_dir": dir,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected both .go files in recursive glob, got: %q", out)
	}
	if strings.Contains(out, "c.ts") {
		t.Errorf("should not include .ts file, got: %q", out)
	}
}

func TestGlobDoublestarSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.Mkdir(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte{}, 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte{}, 0o644)

	g := NewGlobTool()
	argsJSON, _ := json.Marshal(map[string]any{
		"pattern":  "**",
		"base_dir": dir,
	})
	out, err := g.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ".git") {
		t.Errorf("should skip .git in recursive glob, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// ToolSearchTool tests
// ---------------------------------------------------------------------------

func TestToolSearchArgInfoDrillsIntoProperties(t *testing.T) {
	// tool_search must score tools by argument name/description, not just
	// tool name and description. Previously the arg extraction iterated the
	// top-level Parameters map (which only has "type","properties","required")
	// and skipped all of them, so argInfo was always empty.
	r := New()
	ts := NewToolSearchTool(r)

	// "path" is an argument name present in read_file, grep, glob, show_file_tree.
	argsJSON, _ := json.Marshal(map[string]any{
		"query": "path",
		"mode":  "bm25",
	})
	out, err := ts.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	// At least one file-related tool should be found.
	if !strings.Contains(out, "tool_references") {
		t.Errorf("expected tool_references in output, got: %q", out)
	}
	// Should return results, not an empty list.
	if strings.Contains(out, `"tool_references":[]`) {
		t.Errorf("arg-based search returned no results; argInfo extraction may still be broken: %q", out)
	}
}

func TestToolSearchReturnsReferences(t *testing.T) {
	r := New()
	ts := NewToolSearchTool(r)

	argsJSON, _ := json.Marshal(map[string]any{
		"query": "web_search",
		"mode":  "regex",
	})
	out, err := ts.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "web_search") {
		t.Errorf("expected web_search in results, got: %q", out)
	}
}

func TestToolSearchDoesNotReturnSelf(t *testing.T) {
	r := New()
	ts := NewToolSearchTool(r)

	argsJSON, _ := json.Marshal(map[string]any{
		"query": "tool_search",
		"mode":  "regex",
	})
	out, err := ts.Execute(string(argsJSON))
	if err != nil {
		t.Fatal(err)
	}
	// tool_search itself must never appear in its own results.
	if strings.Contains(out, `"tool_name":"tool_search"`) {
		t.Errorf("tool_search must not appear in its own search results: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Registry DeferredDefinitions tests
// ---------------------------------------------------------------------------

func TestDeferredDefinitionsWorksWithProductionRegistry(t *testing.T) {
	// After the RegisterHidden→Register fix, DeferredDefinitions() must return
	// all deferred tools from the production registry.
	r := New()
	deferred := r.DeferredDefinitions()
	if len(deferred) == 0 {
		t.Fatal("DeferredDefinitions() returned empty for production registry; RegisterHidden→Register fix may be missing")
	}
	seen := map[string]bool{}
	for _, d := range deferred {
		seen[d.Name] = true
		if !d.DeferLoading {
			t.Errorf("tool %q in DeferredDefinitions() has DeferLoading=false", d.Name)
		}
	}
	for _, expected := range []string{"web_search", "fetch_url", "read_file", "glob", "grep"} {
		if !seen[expected] {
			t.Errorf("expected deferred tool %q not found in DeferredDefinitions()", expected)
		}
	}
}

func TestToolSearchToolNotDeferred(t *testing.T) {
	// The spec: the tool_search tool itself must never have DeferLoading=true.
	r := New()
	def, ok := r.DefinitionByName(ToolSearchToolName)
	if !ok {
		t.Fatal("tool_search not registered")
	}
	if def.DeferLoading {
		t.Error("tool_search must not have DeferLoading=true")
	}
}
