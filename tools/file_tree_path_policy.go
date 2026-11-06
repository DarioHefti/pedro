package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// fileTreePathPolicy validates a listing root before any directory is read.
type fileTreePathPolicy interface {
	ValidateRoot(string) (string, error)
}

type privateFileTreePathPolicy struct {
	homeDir func() (string, error)
}

func newFileTreePathPolicy() fileTreePathPolicy {
	return privateFileTreePathPolicy{homeDir: os.UserHomeDir}
}

func (p privateFileTreePathPolicy) ValidateRoot(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("access denied: show_file_tree requires an absolute path to a specific folder; relative paths such as '.' are not allowed. Ask the user for the folder path instead of exploring the computer")
	}
	path = filepath.Clean(path)
	home, err := p.homeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", fmt.Errorf("access denied: cannot establish home-directory privacy boundaries for show_file_tree")
	}
	home = filepath.Clean(home)
	if isBroadFileTreeRoot(path, home) {
		return "", fileTreeAccessDenied()
	}
	// Resolve aliases before listing: a symlink to / or a home directory must
	// not turn a seemingly specific folder into unrestricted exploration.
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", fmt.Errorf("access denied: cannot resolve home-directory privacy boundaries for show_file_tree")
	}
	if isBroadFileTreeRoot(realPath, home) || isBroadFileTreeRoot(realPath, realHome) {
		return "", fileTreeAccessDenied()
	}
	return realPath, nil
}

func fileTreeAccessDenied() error {
	return fmt.Errorf("access denied: show_file_tree is not allowed to list filesystem roots, top-level system directories, user home directories, or their parent directories. Do not explore the computer or retry through aliases. Ask the user for a specific folder path")
}

func isBroadFileTreeRoot(path, home string) bool {
	path = filepath.Clean(path)
	// Covers filesystem/drive/share roots and top-level directories, including
	// /Users, /home, /Volumes and C:\Users.
	parent := filepath.Dir(path)
	if parent == path || filepath.Dir(parent) == parent {
		return true
	}
	for ancestor := filepath.Clean(home); ; ancestor = filepath.Dir(ancestor) {
		if sameFileTreePath(path, ancestor) {
			return true
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	// Protect other users' home directories as well as the current user's.
	volumeRoot := filepath.VolumeName(path) + string(filepath.Separator)
	for _, container := range []string{"Users", "home", "Volumes"} {
		if sameFileTreePath(parent, filepath.Join(volumeRoot, container)) {
			return true
		}
	}
	return false
}

func sameFileTreePath(a, b string) bool {
	// macOS commonly uses case-insensitive filesystems too. Conservatively
	// reject case variants even when a particular volume is case-sensitive.
	if filepath.Separator == '\\' || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
