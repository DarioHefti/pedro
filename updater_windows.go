//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	windowsUninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Pedro CorpPedro`
	windowsCompanyName  = "Pedro Corp"
	windowsProductName  = "Pedro"
)

func getWindowsInstallDir() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsUninstallKey, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if dir, _, err := k.GetStringValue("InstallLocation"); err == nil {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				return dir
			}
		}
	}

	if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
		return filepath.Join(programFiles, windowsCompanyName, windowsProductName)
	}

	return filepath.Join(`C:\Program Files`, windowsCompanyName, windowsProductName)
}

func (u *Updater) GetWindowsDefenderExclusion() string {
	dir := getWindowsInstallDir()
	return `Add-MpPreference -AttackSurfaceReductionOnlyExclusions "` + dir + `\pedro.exe"`
}
