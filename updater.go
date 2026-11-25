package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set at build time via -ldflags.
var Version = "dev"

const (
	// releaseDownloadPrefix is where Pedro's release files are served; the updater downloads
	// nothing from anywhere else.
	releaseDownloadPrefix = "https://github.com/DarioHefti/pedro/releases/download/"
	// checksumsAssetName is the release file listing the SHA-256 of every other release file.
	checksumsAssetName = "SHA256SUMS"
	// maxChecksumsBytes bounds the checksum file download.
	maxChecksumsBytes = 1 << 20
)

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
	ctx         context.Context
	mu          sync.Mutex
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
	u.logUpdate("Checking for updates...")
	release, err := fetchLatestRelease()
	if err != nil {
		u.logUpdate(fmt.Sprintf("Failed to check for updates: %v", err))
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(Version, "v")

	u.logUpdate(fmt.Sprintf("Current version: %s, Latest version: %s", currentVersion, latestVersion))

	info := &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseURL:     release.HTMLURL,
	}

	if !isNewerVersion(currentVersion, latestVersion) {
		u.logUpdate("No newer version available")
		info.Available = false
		return info, nil
	}

	asset := pickAsset(release.Assets)
	if asset == nil {
		u.logUpdate("No compatible asset found")
		return nil, fmt.Errorf("no compatible asset found for %s/%s", goruntime.GOOS, goruntime.GOARCH)
	}

	u.logUpdate(fmt.Sprintf("Update available! Asset: %s", asset.Name))
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

	u.logUpdate("Starting download and install process")
	// The name comes from the URL, not from assetName: both arrive from the frontend, and
	// only a file of one of Pedro's releases may be downloaded and run.
	name, checksumsURL, err := releaseAssetURLs(assetURL)
	if err != nil {
		u.logUpdate(fmt.Sprintf("Refusing update download: %v", err))
		runtime.EventsEmit(u.ctx, "update_progress", -1, "error")
		return err
	}
	if name != assetName {
		u.logUpdate(fmt.Sprintf("Asset name %q differs from the URL's %q; using the URL's", assetName, name))
	}
	destPath := filepath.Join(os.TempDir(), name)

	u.logUpdate(fmt.Sprintf("Downloading from: %s", assetURL))
	runtime.EventsEmit(u.ctx, "update_progress", 0, "downloading")

	if err := downloadVerified(u.ctx, assetURL, checksumsURL, name, destPath); err != nil {
		u.logUpdate(fmt.Sprintf("Download failed: %v", err))
		runtime.EventsEmit(u.ctx, "update_progress", -1, "error")
		return fmt.Errorf("download failed: %w", err)
	}

	u.logUpdate("Download successful, SHA-256 verified")
	runtime.EventsEmit(u.ctx, "update_progress", 100, "installing")

	u.logUpdate(fmt.Sprintf("Launching installer: %s", destPath))
	if err := launchInstaller(destPath); err != nil {
		u.logUpdate(fmt.Sprintf("Install failed: %v", err))
		runtime.EventsEmit(u.ctx, "update_progress", -1, "error")
		return fmt.Errorf("install failed: %w", err)
	}

	u.logUpdate("Install launched")
	runtime.EventsEmit(u.ctx, "update_progress", 100, "done")

	// Deferred install scripts wait for Pedro to exit before replacing files.
	if goruntime.GOOS == "windows" || goruntime.GOOS == "darwin" || goruntime.GOOS == "linux" {
		runtime.Quit(u.ctx)
	}

	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	url := "https://api.github.com/repos/DarioHefti/pedro/releases/latest"
	logUpdate(fmt.Sprintf("Fetching latest release from: %s", url))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logUpdate(fmt.Sprintf("Failed to create request: %v", err))
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pedro-Updater/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logUpdate(fmt.Sprintf("HTTP request failed: %v", err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logUpdate(fmt.Sprintf("GitHub API returned status %d", resp.StatusCode))
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		logUpdate(fmt.Sprintf("Failed to decode GitHub release JSON: %v", err))
		return nil, err
	}
	return &release, nil
}

func pickAsset(assets []githubAsset) *githubAsset {
	os := goruntime.GOOS
	arch := goruntime.GOARCH

	logUpdate(fmt.Sprintf("Picking asset for OS: %s, Arch: %s", os, arch))
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
		logUpdate(fmt.Sprintf("Selected preferred asset: %s", preferred.Name))
		return preferred
	}
	if fallback != nil {
		logUpdate(fmt.Sprintf("Selected fallback asset: %s", fallback.Name))
		return fallback
	}

	logUpdate("No compatible asset found")
	return nil
}

// releaseAssetURLs checks that assetURL is a file of one of Pedro's GitHub releases and
// returns the file's name and the URL of that release's checksum file.
func releaseAssetURLs(assetURL string) (name, checksumsURL string, err error) {
	if len(assetURL) <= len(releaseDownloadPrefix) || !strings.EqualFold(assetURL[:len(releaseDownloadPrefix)], releaseDownloadPrefix) {
		return "", "", fmt.Errorf("not a Pedro release download: %s", assetURL)
	}
	tag, name, ok := strings.Cut(assetURL[len(releaseDownloadPrefix):], "/")
	if !ok || tag == "" || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\?#%") {
		return "", "", fmt.Errorf("not a Pedro release download: %s", assetURL)
	}
	return name, assetURL[:len(releaseDownloadPrefix)] + tag + "/" + checksumsAssetName, nil
}

// downloadVerified downloads assetURL to dest and checks its SHA-256 against the one the
// release's checksum file lists for name. A file that does not match is deleted.
func downloadVerified(ctx context.Context, assetURL, checksumsURL, name, dest string) error {
	want, err := expectedChecksum(ctx, checksumsURL, name)
	if err != nil {
		return fmt.Errorf("cannot verify the update: %w", err)
	}
	got, err := downloadFile(ctx, assetURL, dest)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		_ = os.Remove(dest)
		return fmt.Errorf("the downloaded %s is not the published file (SHA-256 %s, expected %s)", name, got, want)
	}
	return nil
}

// expectedChecksum fetches a release's checksum file and returns the SHA-256 it lists for name.
func expectedChecksum(ctx context.Context, checksumsURL, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", checksumsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Pedro-Updater/"+Version)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the release has no %s file (status %d)", checksumsAssetName, resp.StatusCode)
	}
	sums, err := parseChecksums(io.LimitReader(resp.Body, maxChecksumsBytes))
	if err != nil {
		return "", err
	}
	sum, ok := sums[name]
	if !ok {
		return "", fmt.Errorf("%s lists no checksum for %s", checksumsAssetName, name)
	}
	return sum, nil
}

// parseChecksums reads sha256sum output: one "<hex>  <name>" line per file ("*<name>" in
// binary mode).
func parseChecksums(r io.Reader) (map[string]string, error) {
	sums := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		sum, name := strings.ToLower(fields[0]), strings.TrimPrefix(fields[1], "*")
		if decoded, err := hex.DecodeString(sum); err != nil || len(decoded) != sha256.Size {
			continue
		}
		sums[name] = sum
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(sums) == 0 {
		return nil, errors.New(checksumsAssetName + " lists no checksums")
	}
	return sums, nil
}

// downloadFile downloads url to dest and returns the file's SHA-256 as hex.
func downloadFile(ctx context.Context, url, dest string) (string, error) {
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove stale download: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Pedro-Updater/"+Version)
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, hash), resp.Body)
	if err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	if written == 0 {
		_ = os.Remove(dest)
		return "", fmt.Errorf("download returned empty file")
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
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

func logUpdate(message string) {
	// In Pedro's data folder: a relative path would land in whatever folder Pedro was started from.
	f, err := os.OpenFile(filepath.Join(appDataDir(), "updater.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "[%s] %s\n", timestamp, message)
}

func (u *Updater) logUpdate(message string) {
	logUpdate(message)
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var result []int
	for _, part := range parts {
		if i, err := strconv.Atoi(part); err == nil {
			result = append(result, i)
		} else {
			result = append(result, 0)
		}
	}
	for len(result) < 3 {
		result = append(result, 0)
	}
	return result
}
