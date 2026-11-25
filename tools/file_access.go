package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// FileAccess decides which local files the file tools may read. Web pages and other tool
// results can carry instructions, so the model must not be able to read anything the person
// did not hand to the chat.
type FileAccess interface {
	// Resolve checks that path may be read and returns it with symlinks resolved, which is the
	// path to open. The error tells the model why not.
	Resolve(path string) (string, error)
}

type fileAccessKey struct{}

// WithFileAccess returns a context whose tool calls may read what access allows.
func WithFileAccess(ctx context.Context, access FileAccess) context.Context {
	return context.WithValue(ctx, fileAccessKey{}, access)
}

// fileAccessFrom is the access a tool call has. Without one nothing may be read.
func fileAccessFrom(ctx context.Context) FileAccess {
	if access, ok := ctx.Value(fileAccessKey{}).(FileAccess); ok && access != nil {
		return access
	}
	return AllowList{}
}

// AllowList allows single files and whole folders (with everything below them).
type AllowList struct {
	files []string
	dirs  []string
}

// NewAllowList allows the given files and folders. Paths that are not absolute are ignored.
func NewAllowList(files, dirs []string) AllowList {
	var list AllowList
	for _, f := range files {
		list.files = append(list.files, allowedForms(f)...)
	}
	for _, d := range dirs {
		list.dirs = append(list.dirs, allowedForms(d)...)
	}
	return list
}

// allowedForms is a root as given and with its symlinks resolved (e.g. /tmp and /private/tmp
// on macOS), so either spelling of an allowed path matches.
func allowedForms(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return nil
	}
	path = filepath.Clean(path)
	forms := []string{path}
	if real, err := filepath.EvalSymlinks(path); err == nil && real != path {
		forms = append(forms, real)
	}
	return forms
}

// errNotAllowed is what the model is told when a path is outside what it may read.
var errNotAllowed = errors.New("access denied: only files and folders the user attached to this chat can be read. Ask the user to attach the file or folder (or to paste its path into the chat) instead of trying other paths")

func (l AllowList) Resolve(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute: %q", path)
	}
	path = filepath.Clean(path)
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Report a missing file only when the person could have meant it, so the model cannot
		// probe the rest of the disk.
		if l.allows(path) {
			return "", err
		}
		return "", errNotAllowed
	}
	// The resolved path decides: a symlink in an attached folder must not lead out of it.
	if !l.allows(real) {
		return "", errNotAllowed
	}
	return real, nil
}

func (l AllowList) allows(path string) bool {
	for _, f := range l.files {
		if samePath(path, f) {
			return true
		}
	}
	for _, d := range l.dirs {
		if isWithin(path, d) {
			return true
		}
	}
	return false
}

// isWithin reports whether path is dir or below it.
func isWithin(path, dir string) bool {
	if samePath(path, dir) {
		return true
	}
	prefix := dir
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if caseInsensitivePaths() {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

func samePath(a, b string) bool {
	if caseInsensitivePaths() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// caseInsensitivePaths: Windows and macOS file systems usually ignore case, so a case variant
// names the same, allowed, file.
func caseInsensitivePaths() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}
