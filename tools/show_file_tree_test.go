package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileTreePolicyBlocksBroadRoots(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	home := filepath.Join(root, "Users", "example", "nested-home")
	policy := privateFileTreePathPolicy{homeDir: func() (string, error) { return home, nil }}
	for _, path := range []string{
		".", "..", "relative/folder", root,
		filepath.Join(root, "Users"), filepath.Join(root, "home"),
		filepath.Join(root, "etc"), filepath.Join(root, "Volumes", "disk"),
		home, filepath.Dir(home), filepath.Dir(filepath.Dir(home)),
		filepath.Join(root, "Users", "someone-else"),
		filepath.Join(root, "home", "someone-else"),
		home + string(filepath.Separator) + ".",
		home + string(filepath.Separator) + "project" + string(filepath.Separator) + "..",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := policy.ValidateRoot(path)
			if err == nil || !strings.Contains(err.Error(), "access denied") {
				t.Fatalf("expected explicit access denial for %q, got %v", path, err)
			}
		})
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if _, err := policy.ValidateRoot(strings.ToUpper(home)); err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Fatalf("case variants must not bypass home-directory denial: %v", err)
		}
	}
}

func TestFileTreePolicyResolvesSymlinks(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	policy := privateFileTreePathPolicy{homeDir: func() (string, error) { return home, nil }}
	for _, tc := range []struct {
		name   string
		target string
		deny   bool
	}{
		{"home-alias", home, true},
		{"parent-alias", base, true},
		{"root-alias", filepath.VolumeName(base) + string(filepath.Separator), true},
		{"project-alias", project, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alias := filepath.Join(project, tc.name)
			if err := os.Symlink(tc.target, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			_, err := policy.ValidateRoot(alias)
			if tc.deny {
				if err == nil || !strings.Contains(err.Error(), "access denied") {
					t.Fatalf("expected access denial, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("specific project alias should be allowed: %v", err)
			}
		})
	}
}

func TestFileTreePolicyFailsClosedWithoutHome(t *testing.T) {
	policy := privateFileTreePathPolicy{homeDir: func() (string, error) {
		return "", os.ErrNotExist
	}}
	if _, err := policy.ValidateRoot(t.TempDir()); err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected access denial without privacy boundaries, got %v", err)
	}
}

func TestShowFileTreeDenialThroughRegistry(t *testing.T) {
	registry := newTestRegistry()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".", home, filepath.Dir(home)} {
		args, err := json.Marshal(map[string]any{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		out := registry.Execute("show_file_tree", string(args))
		if !strings.Contains(out, "access denied") || !strings.Contains(out, "Ask the user") {
			t.Fatalf("expected clear tool-level refusal for %q, got %q", path, out)
		}
		if strings.Contains(out, "Root:") || strings.Contains(out, "├──") || strings.Contains(out, "└──") {
			t.Fatalf("denied call must not return directory contents: %q", out)
		}
	}
}

func TestShowFileTreeSpecificFolderAndPagination(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("example"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tool := NewShowFileTreeTool()
	for _, tc := range []struct {
		offset int
		want   string
		absent string
	}{
		{1, "a.txt", "b.txt"},
		{2, "b.txt", "a.txt"},
	} {
		args, err := json.Marshal(map[string]any{"path": dir, "depth": 1, "limit": 1, "offset": tc.offset})
		if err != nil {
			t.Fatal(err)
		}
		out, err := tool.Execute(string(args))
		if err != nil || !strings.Contains(out, tc.want) || strings.Contains(out, tc.absent) {
			t.Fatalf("expected successful paginated folder listing, got %q, %v", out, err)
		}
	}
}
