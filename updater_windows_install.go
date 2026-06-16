//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchInstaller(path string) error {
	installDir := getWindowsInstallDir()

	scriptPath := filepath.Join(os.TempDir(), "pedro-update.ps1")
	script := fmt.Sprintf(`Start-Sleep -Seconds 2
Start-Process -FilePath %q -ArgumentList '/S','/D=%s' -Verb RunAs
`, path, installDir)

	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	return cmd.Start()
}
