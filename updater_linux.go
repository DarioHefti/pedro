//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchInstaller(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	// os.Rename from $TMPDIR fails when temp and install dir are on different
	// mounts; install(1) copies atomically after Pedro exits.
	scriptPath := filepath.Join(os.TempDir(), "pedro-update.sh")
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail
sleep 2
install -m 755 %q %q
`, path, exe)

	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	return cmd.Start()
}
