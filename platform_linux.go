//go:build linux

package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// linuxAppID is the GTK program name. On Wayland it becomes the xdg app-id, which compositors
// match against pedro.desktop for the window icon and which window rules (e.g. Hyprland's
// class:pedro) target. Without it the id is the binary's file name, e.g. "Pedro-linux-amd64".
const linuxAppID = "pedro"

//go:embed build/appicon.png
var linuxIcon []byte

// linuxEnv is the slice of the process environment the Linux tweaks depend on, so they can
// be tested without touching the real environment or hardware.
type linuxEnv interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
	Setenv(key, value string) error
	// HasNvidiaDriver reports whether the proprietary NVIDIA kernel driver is loaded.
	HasNvidiaDriver() bool
}

type osLinuxEnv struct{}

func (osLinuxEnv) Getenv(key string) string            { return os.Getenv(key) }
func (osLinuxEnv) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
func (osLinuxEnv) Setenv(key, value string) error      { return os.Setenv(key, value) }
func (osLinuxEnv) HasNvidiaDriver() bool {
	_, err := os.Stat("/proc/driver/nvidia/version")
	return err == nil
}

func applyPlatformOptions(opts *options.App) {
	configureLinux(opts, osLinuxEnv{})
}

func configureLinux(opts *options.App, env linuxEnv) {
	opts.Linux = &linux.Options{
		ProgramName:      linuxAppID,
		Icon:             linuxIcon, // used by X11; Wayland takes the icon from pedro.desktop
		WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
	}

	if isWayland(env) {
		// Tiling compositors (Hyprland, Sway, niri) size windows themselves. A min size larger
		// than the tile makes GTK commit an oversized surface that the compositor then shrinks,
		// leaving the UI blurry and mis-laid-out. The layout is responsive, so let the tile win.
		opts.MinWidth = 0
		opts.MinHeight = 0
	}

	// WebKitGTK's DMA-BUF renderer shows a blank window or dies with "Error 71 (Protocol
	// error)" on the NVIDIA driver.
	if env.HasNvidiaDriver() {
		setDefaultEnv(env, "WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	// With accelerated compositing on Wayland (seen with WebKitGTK 2.54 on Hyprland, Intel),
	// WebKit intermittently presents a stale frame for one refresh: a closed dialog or a
	// half-faded message flashes back, most visibly while sending and streaming a reply.
	// Painting without compositing layers removes it at no visible speed cost for this UI,
	// whereas disabling the DMA-BUF renderer instead made streaming repaint far less often.
	if isWayland(env) {
		setDefaultEnv(env, "WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	}
}

// setDefaultEnv sets key unless the user already chose a value (e.g. "0" to opt out).
func setDefaultEnv(env linuxEnv, key, value string) {
	if _, set := env.LookupEnv(key); !set {
		_ = env.Setenv(key, value)
	}
}

// isWayland reports whether GTK will run as a native Wayland client. Wails itself forces
// X11 only when no Wayland session is detected, so GDK_BACKEND=x11 is the user opting out.
func isWayland(env linuxEnv) bool {
	if env.Getenv("WAYLAND_DISPLAY") == "" && env.Getenv("XDG_SESSION_TYPE") != "wayland" {
		return false
	}
	backend := env.Getenv("GDK_BACKEND")
	return backend == "" || backend == "wayland" || strings.HasPrefix(backend, "wayland,")
}
