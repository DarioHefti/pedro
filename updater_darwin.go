//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func launchInstaller(path string) error {
	targetApp, err := currentAppBundlePath()
	if err != nil {
		return err
	}

	lowerPath := strings.ToLower(path)
	var script string
	switch {
	case strings.HasSuffix(lowerPath, ".dmg"):
		script = darwinDMGUpdateScript(path, targetApp)
	case strings.HasSuffix(lowerPath, ".zip"):
		script = darwinZipUpdateScript(path, targetApp)
	default:
		return fmt.Errorf("unsupported macOS update package: %s", filepath.Base(path))
	}

	scriptPath := filepath.Join(os.TempDir(), "pedro-update.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	return cmd.Start()
}

func currentAppBundlePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}

	// .../pedro.app/Contents/MacOS/pedro
	macosDir := filepath.Dir(exe)
	if filepath.Base(macosDir) != "MacOS" {
		return "/Applications/pedro.app", nil
	}
	contentsDir := filepath.Dir(macosDir)
	if filepath.Base(contentsDir) != "Contents" {
		return "/Applications/pedro.app", nil
	}
	appBundle := filepath.Dir(contentsDir)
	if !strings.HasSuffix(strings.ToLower(filepath.Base(appBundle)), ".app") {
		return "/Applications/pedro.app", nil
	}
	return appBundle, nil
}

func darwinDMGUpdateScript(dmgPath, targetApp string) string {
	mountPoint := filepath.Join(os.TempDir(), "pedro-dmg-mount")
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
sleep 2
MOUNT=%q
mkdir -p "$MOUNT"
if ! hdiutil attach -nobrowse -readonly -mountpoint "$MOUNT" %q; then
  echo "failed to mount update DMG" >&2
  exit 1
fi
cleanup() {
  hdiutil detach "$MOUNT" -quiet 2>/dev/null || true
  rmdir "$MOUNT" 2>/dev/null || true
}
trap cleanup EXIT
SOURCE="$MOUNT/pedro.app"
if [ ! -d "$SOURCE" ]; then
  SOURCE=$(find "$MOUNT" -maxdepth 1 -name '*.app' -type d | head -1)
fi
if [ -z "$SOURCE" ] || [ ! -d "$SOURCE" ]; then
  echo "pedro.app not found in DMG" >&2
  exit 1
fi
rm -rf %q
ditto "$SOURCE" %q
xattr -dr com.apple.quarantine %q 2>/dev/null || true
`, mountPoint, dmgPath, targetApp, targetApp, targetApp)
}

func darwinZipUpdateScript(zipPath, targetApp string) string {
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
sleep 2
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
unzip -q %q -d "$WORKDIR"
SOURCE=$(find "$WORKDIR" -maxdepth 2 -name '*.app' -type d | head -1)
if [ -z "$SOURCE" ] || [ ! -d "$SOURCE" ]; then
  echo "pedro.app not found in zip" >&2
  exit 1
fi
rm -rf %q
ditto "$SOURCE" %q
xattr -dr com.apple.quarantine %q 2>/dev/null || true
`, zipPath, targetApp, targetApp, targetApp)
}
