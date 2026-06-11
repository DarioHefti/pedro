package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
	HTMLURL string        `json:"html_url"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseURL"`
	AssetName      string `json:"assetName"`
	AssetURL       string `json:"assetURL"`
}

type Updater struct {
	ctx        context.Context
	mu         sync.Mutex
	downloading bool
}

func NewUpdater() *Updater {
	return &Updater{}
}

func (u *Updater) startup(ctx context.Context) {
	u.ctx = ctx
}

func (u *Updater) GetCurrentVersion() string {
	return Version
}

func (u *Updater) CheckForUpdate() (*UpdateInfo, error) {
	release, err := fetchLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(Version, "v")

	info := &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseURL:     release.HTMLURL,
	}

	if !isNewerVersion(currentVersion, latestVersion) {
		info.Available = false
		return info, nil
	}

	asset := pickAsset(release.Assets)
	if asset == nil {
		return nil, fmt.Errorf("no compatible asset found for %s/%s", goruntime.GOOS, goruntime.GOARCH)
	}

	info.Available = true
	info.AssetName = asset.Name
	info.AssetURL = asset.BrowserDownloadURL
	return info, nil
}

func (u *Updater) DownloadAndInstall(assetURL, assetName string) error {
	u.mu.Lock()
	if u.downloading {
		u.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	u.downloading = true
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.downloading = false
		u.mu.Unlock()
	}()

	tmpDir := os.TempDir()
	destPath := filepath.Join(tmpDir, assetName)

	runtime.EventsEmit(u.ctx, "update_progress", 0, "downloading")

	if err := downloadFile(u.ctx, assetURL, destPath); err != nil {
		runtime.EventsEmit(u.ctx, "update_progress", -1, "error")
		return fmt.Errorf("download failed: %w", err)
	}

	runtime.EventsEmit(u.ctx, "update_progress", 100, "installing")

	if err := launchInstaller(destPath); err != nil {
		runtime.EventsEmit(u.ctx, "update_progress", -1, "error")
		return fmt.Errorf("install failed: %w", err)
	}

	runtime.EventsEmit(u.ctx, "update_progress", 100, "done")

	// On Windows the NSIS installer kills the running process via taskkill,
	// but we also quit gracefully here so the user sees a clean shutdown.
	if goruntime.GOOS == "windows" {
		runtime.Quit(u.ctx)
	}

	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	url := "https://api.github.com/repos/DarioHefti/pedro/releases/latest"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pedro-Updater/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func pickAsset(assets []githubAsset) *githubAsset {
	os := goruntime.GOOS
	arch := goruntime.GOARCH

	var preferred, fallback *githubAsset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)

		switch os {
		case "windows":
			if strings.Contains(name, "windows") && strings.Contains(name, arch) {
				if strings.Contains(name, "installer") {
					preferred = &assets[i]
				} else if fallback == nil {
					fallback = &assets[i]
				}
			}
		case "darwin":
			if strings.Contains(name, "macos") && strings.Contains(name, arch) {
				if strings.Contains(name, ".dmg") {
					preferred = &assets[i]
				} else if fallback == nil {
					fallback = &assets[i]
				}
			}
		case "linux":
			if strings.Contains(name, "linux") && strings.Contains(name, arch) {
				preferred = &assets[i]
			}
		}
	}

	if preferred != nil {
		return preferred
	}
	return fallback
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func launchInstaller(path string) error {
	switch goruntime.GOOS {
	case "windows":
		cmd := exec.Command(path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "darwin":
		if strings.HasSuffix(path, ".dmg") {
			return exec.Command("open", path).Start()
		}
		return exec.Command("open", path).Start()
	case "linux":
		if err := os.Chmod(path, 0755); err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		// Replace current binary
		return os.Rename(path, exe)
	default:
		return fmt.Errorf("unsupported OS: %s", goruntime.GOOS)
	}
}

// isNewerVersion returns true if latest > current (simple semver comparison).
func isNewerVersion(current, latest string) bool {
	if current == "dev" || current == "" {
		return true
	}

	currentParts := parseSemver(current)
	latestParts := parseSemver(latest)

	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.Split(parts[i], "-")[0])
		result[i] = n
	}
	return result
}
