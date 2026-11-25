package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allowEverything lets the tests of the tools themselves read any path.
type allowEverything struct{}

func (allowEverything) Resolve(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", os.ErrInvalid
	}
	return filepath.Clean(path), nil
}

// openFiles is a tool-call context that may read every file.
func openFiles(t *testing.T) context.Context {
	return WithFileAccess(t.Context(), allowEverything{})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAllowListAllowsAttachedFilesAndFolders(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "report.txt")
	folder := filepath.Join(base, "project")
	nested := filepath.Join(folder, "src", "main.go")
	sibling := filepath.Join(base, "secret.txt")
	writeFile(t, file, "r")
	writeFile(t, nested, "m")
	writeFile(t, sibling, "s")
	list := NewAllowList([]string{file}, []string{folder})

	for _, ok := range []string{file, folder, nested, filepath.Join(folder, "src", "..", "src", "main.go")} {
		if _, err := list.Resolve(ok); err != nil {
			t.Errorf("Resolve(%q) = %v, want allowed", ok, err)
		}
	}
	for _, denied := range []string{sibling, base, filepath.Join(folder, "..", "secret.txt"), folder + "-other"} {
		if _, err := list.Resolve(denied); err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Errorf("Resolve(%q) = %v, want access denied", denied, err)
		}
	}
	// A missing file outside reads as denied, so the model cannot probe what exists.
	if _, err := list.Resolve(filepath.Join(base, "missing")); err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("missing outside: %v", err)
	}
	if _, err := list.Resolve(filepath.Join(folder, "missing")); err == nil || strings.Contains(err.Error(), "access denied") {
		t.Errorf("missing inside should be a plain not-found error: %v", err)
	}
	if _, err := list.Resolve("relative/path"); err == nil {
		t.Error("a relative path was allowed")
	}
}

func TestAllowListFollowsSymlinksOnlyWithinWhatIsAllowed(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "project")
	secret := filepath.Join(base, "secret.txt")
	writeFile(t, filepath.Join(folder, "a.txt"), "a")
	writeFile(t, secret, "s")
	link := filepath.Join(folder, "escape.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	inner := filepath.Join(folder, "inner.txt")
	if err := os.Symlink(filepath.Join(folder, "a.txt"), inner); err != nil {
		t.Fatal(err)
	}
	list := NewAllowList(nil, []string{folder})

	if _, err := list.Resolve(link); err == nil {
		t.Fatal("a symlink out of the attached folder was followed")
	}
	if _, err := list.Resolve(inner); err != nil {
		t.Fatalf("a symlink within the folder: %v", err)
	}
}

func TestFileToolsDenyWithoutAccess(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	writeFile(t, file, "needle")
	ctx := WithFileAccess(t.Context(), NewAllowList(nil, nil))
	path, _ := json.Marshal(file)
	folder, _ := json.Marshal(dir)

	for name, run := range map[string]func() (string, error){
		"read_file":      func() (string, error) { return NewReadFileTool().ExecuteContext(ctx, `{"path":`+string(path)+`}`) },
		"parse_document": func() (string, error) { return NewParseDocumentTool().ExecuteContext(ctx, `{"path":`+string(path)+`}`) },
		"grep":           func() (string, error) { return NewGrepTool().ExecuteContext(ctx, `{"pattern":"needle","path":`+string(folder)+`}`) },
		"glob":           func() (string, error) { return NewGlobTool().ExecuteContext(ctx, `{"pattern":"*.txt","base_dir":`+string(folder)+`}`) },
		"show_file_tree": func() (string, error) { return NewShowFileTreeTool().ExecuteContext(ctx, `{"path":`+string(folder)+`}`) },
		// Without any access in the context, nothing is readable either.
		"read_file without access": func() (string, error) {
			return NewReadFileTool().ExecuteContext(context.Background(), `{"path":`+string(path)+`}`)
		},
	} {
		out, err := run()
		if err != nil {
			out = err.Error()
		}
		if !strings.Contains(out, "access denied") || strings.Contains(out, "needle") {
			t.Errorf("%s: got %q, want access denied", name, out)
		}
	}
}

func TestGrepAndGlobStayWithinTheAttachedFolder(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "project")
	writeFile(t, filepath.Join(folder, "a.txt"), "needle inside")
	writeFile(t, filepath.Join(base, "secret.txt"), "needle outside")
	if err := os.Symlink(filepath.Join(base, "secret.txt"), filepath.Join(folder, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	ctx := WithFileAccess(t.Context(), NewAllowList(nil, []string{folder}))
	dir, _ := json.Marshal(folder)

	out, _ := NewGrepTool().ExecuteContext(ctx, `{"pattern":"needle","path":`+string(dir)+`}`)
	if !strings.Contains(out, "needle inside") || strings.Contains(out, "needle outside") {
		t.Errorf("grep = %q", out)
	}
	out, _ = NewGlobTool().ExecuteContext(ctx, `{"pattern":"../*.txt","base_dir":`+string(dir)+`}`)
	if strings.Contains(out, "secret.txt") {
		t.Errorf("glob listed a file outside the folder: %q", out)
	}
	out, _ = NewGlobTool().ExecuteContext(ctx, `{"pattern":"*.txt","base_dir":`+string(dir)+`}`)
	if !strings.Contains(out, "a.txt") || strings.Contains(out, "link.txt") {
		t.Errorf("glob = %q, want a.txt and not the symlink out", out)
	}
}
