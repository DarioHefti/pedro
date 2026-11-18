// Package browser drives a local headless Chrome (via go-rod) for pages that
// need JavaScript, solve bot challenges, or should be navigated interactively.
package browser

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

// Rendered is the state of a page after it was rendered in the browser.
type Rendered struct {
	URL   string // final URL after redirects
	Title string
	HTML  string
}

// RenderOptions tunes how long Render waits.
type RenderOptions struct {
	// ReadySelector makes Render return as soon as an element matching this
	// CSS selector exists, instead of waiting for the network to go quiet.
	ReadySelector string
}

// Renderer renders a URL with JavaScript enabled and returns the final DOM.
type Renderer interface {
	Render(ctx context.Context, rawURL string, opts RenderOptions) (*Rendered, error)
}

// Options configures a Manager.
type Options struct {
	// IdleTimeout closes Chrome after this long without any use.
	IdleTimeout time.Duration
	// Logf receives progress messages (e.g. the one-time browser download).
	Logf func(format string, args ...any)
}

// Manager owns one lazily started, stealth-configured headless Chrome that is
// shared by the renderer and interactive sessions. It is safe for concurrent use.
type Manager struct {
	opts Options

	mu       sync.Mutex
	launcher *launcher.Launcher
	browser  *rod.Browser
	identity *proto.NetworkSetUserAgentOverride
	gen      int // incremented on every launch so sessions can detect restarts
	active   int
	lastUsed time.Time
	idle     *time.Timer
}

// NewManager creates a Manager. Chrome is only started on first use.
func NewManager(opts Options) *Manager {
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 10 * time.Minute
	}
	if opts.Logf == nil {
		opts.Logf = func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	}
	return &Manager{opts: opts}
}

// Do runs fn with the shared browser, launching it if needed. gen identifies
// the browser instance; it changes whenever Chrome was restarted.
func (m *Manager) Do(fn func(b *rod.Browser, gen int) error) error {
	b, gen, err := m.acquire()
	if err != nil {
		return err
	}
	defer m.release()
	return fn(b, gen)
}

// NewPage opens a tab with stealth evasions and a non-headless identity.
// The caller owns the page and must close it.
func (m *Manager) NewPage(b *rod.Browser) (*rod.Page, error) {
	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("open tab: %w", err)
	}
	m.mu.Lock()
	identity := m.identity
	m.mu.Unlock()
	if identity != nil {
		_ = page.SetUserAgent(identity)
	}
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1366, Height: 900, DeviceScaleFactor: 1})
	return page, nil
}

// Render implements Renderer.
func (m *Manager) Render(ctx context.Context, rawURL string, opts RenderOptions) (*Rendered, error) {
	var out *Rendered
	err := m.Do(func(b *rod.Browser, _ int) error {
		page, err := m.NewPage(b)
		if err != nil {
			return err
		}
		defer page.Close()

		p := page.Context(ctx)
		if opts.ReadySelector != "" {
			if err := p.Navigate(rawURL); err != nil {
				return fmt.Errorf("navigate: %w", err)
			}
			// A missing element is not an error: the caller inspects the DOM.
			waitReady(p, opts.ReadySelector, 15*time.Second)
		} else if err := Settle(p, func() error { return p.Navigate(rawURL) }); err != nil {
			return fmt.Errorf("navigate: %w", err)
		}

		html, err := RenderedHTML(p)
		if err != nil {
			return fmt.Errorf("read DOM: %w", err)
		}
		info, err := p.Info()
		if err != nil {
			return fmt.Errorf("page info: %w", err)
		}
		out = &Rendered{URL: info.URL, Title: info.Title, HTML: html}
		return nil
	})
	return out, err
}

// Close shuts Chrome down. The manager can be reused afterwards.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownLocked()
	return nil
}

// Settle performs action (a navigation, click, key press, ...) and waits until
// the page finished loading its data, then for non-interactive bot challenges
// (Cloudflare "Just a moment", Anubis proof-of-work, ...) to complete.
// Every wait is bounded so a busy page never blocks the agent.
func Settle(p *rod.Page, action func() error) error {
	waitIdle := p.Timeout(10*time.Second).WaitRequestIdle(700*time.Millisecond, nil, nil, nil)
	if err := action(); err != nil {
		return err
	}
	_ = p.Timeout(15 * time.Second).WaitLoad()
	waitIdle()

	deadline := time.Now().Add(15 * time.Second)
	challenged := false
	for time.Now().Before(deadline) {
		info, err := p.Info()
		if err != nil || !IsChallengeTitle(info.Title) {
			break
		}
		challenged = true
		time.Sleep(500 * time.Millisecond)
	}
	if challenged {
		_ = p.Timeout(15 * time.Second).WaitLoad()
		_ = p.Timeout(5 * time.Second).WaitIdle(5 * time.Second)
	}
	time.Sleep(300 * time.Millisecond) // let the final render frame land
	return nil
}

// RenderedHTML returns the live DOM as HTML. <noscript> fallbacks are dropped
// because JavaScript did run.
func RenderedHTML(p *rod.Page) (string, error) {
	v, err := p.Eval(`() => {
		const doc = document.documentElement.cloneNode(true);
		doc.querySelectorAll('noscript').forEach(n => n.remove());
		return doc.outerHTML;
	}`)
	if err != nil {
		return "", err
	}
	return v.Value.Str(), nil
}

// interactiveChallenge matches captcha widgets that a script cannot solve.
const interactiveChallenge = `[id*="captcha" i], [class*="captcha" i], iframe[src*="captcha"], iframe[src*="challenges.cloudflare.com"]`

// waitReady waits until selector matches, but gives up early when the page
// shows a captcha that needs a human.
func waitReady(p *rod.Page, selector string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if found, _, err := p.Has(selector); err != nil || found {
			return
		}
		if blocked, _, _ := p.Has(interactiveChallenge); blocked {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

var challengeTitles = []string{
	"just a moment",
	"verifying",
	"checking your browser",
	"attention required",
	"making sure you're not a bot",
	"one moment, please",
	"ddos-guard",
}

// IsChallengeTitle reports whether a page title belongs to a bot-check interstitial.
func IsChallengeTitle(title string) bool {
	t := strings.ToLower(title)
	for _, c := range challengeTitles {
		if strings.Contains(t, c) {
			return true
		}
	}
	return false
}

// ── lifecycle ────────────────────────────────────────────────────────────────

func (m *Manager) acquire() (*rod.Browser, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.browser == nil {
		if err := m.launchLocked(); err != nil {
			return nil, 0, err
		}
	}
	m.active++
	if m.idle != nil {
		m.idle.Stop()
	}
	return m.browser, m.gen, nil
}

func (m *Manager) release() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active--
	m.lastUsed = time.Now()
	if m.active == 0 && m.browser != nil {
		m.idle = time.AfterFunc(m.opts.IdleTimeout, m.closeIfIdle)
	}
}

func (m *Manager) closeIfIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == 0 && time.Since(m.lastUsed) >= m.opts.IdleTimeout {
		m.shutdownLocked()
	}
}

func (m *Manager) launchLocked() error {
	bin, err := m.chromeBinary()
	if err != nil {
		return err
	}

	l := launcher.New().
		Bin(bin).
		// Leakless runs Chrome under a tiny guard process that kills it when
		// Pedro dies without a clean shutdown (crash, force quit, dev reload).
		// Windows antivirus tools flag the guard binary, so it is off there.
		Leakless(runtime.GOOS != "windows").
		Headless(true).
		Delete("enable-automation").
		Set("disable-blink-features", "AutomationControlled").
		Set("window-size", "1366,900").
		Set("lang", "en-US")
	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		l.Kill()
		l.Cleanup()
		return fmt.Errorf("connect to browser: %w", err)
	}

	m.launcher = l
	m.browser = b
	m.gen++
	m.identity = identityFor(b)
	return nil
}

func (m *Manager) shutdownLocked() {
	if m.idle != nil {
		m.idle.Stop()
		m.idle = nil
	}
	if m.browser != nil {
		_ = m.browser.Close()
		m.browser = nil
	}
	if m.launcher != nil {
		m.launcher.Kill()
		m.launcher.Cleanup()
		m.launcher = nil
	}
}

func (m *Manager) chromeBinary() (string, error) {
	if path, ok := launcher.LookPath(); ok {
		return path, nil
	}
	m.opts.Logf("[Info] No Chrome/Chromium found. Downloading embedded browser (~150MB, one-time download)...")
	path, err := launcher.NewBrowser().Get()
	if err != nil {
		return "", errors.New("no Chrome/Chromium available and the download failed: " + err.Error() +
			". Please install Chrome manually, or check whether antivirus is blocking the download")
	}
	m.opts.Logf("[Info] Browser download complete!")
	return path, nil
}

// identityFor builds a user agent override that hides "HeadlessChrome" from
// both the User-Agent header and navigator.userAgentData / client hints.
func identityFor(b *rod.Browser) *proto.NetworkSetUserAgentOverride {
	v, err := b.Version()
	if err != nil {
		return nil
	}
	ua := strings.ReplaceAll(v.UserAgent, "HeadlessChrome", "Chrome")

	fullVersion := v.Product
	if i := strings.IndexByte(fullVersion, '/'); i >= 0 {
		fullVersion = fullVersion[i+1:]
	}
	major := fullVersion
	if i := strings.IndexByte(major, '.'); i >= 0 {
		major = major[:i]
	}

	platform, navPlatform := "Windows", "Win32"
	switch runtime.GOOS {
	case "darwin":
		platform, navPlatform = "macOS", "MacIntel"
	case "linux":
		platform, navPlatform = "Linux", "Linux x86_64"
	}
	arch := "x86"
	if runtime.GOARCH == "arm64" {
		arch = "arm"
	}

	brands := []*proto.EmulationUserAgentBrandVersion{
		{Brand: "Not:A-Brand", Version: "99"},
		{Brand: "Google Chrome", Version: major},
		{Brand: "Chromium", Version: major},
	}
	full := []*proto.EmulationUserAgentBrandVersion{
		{Brand: "Not:A-Brand", Version: "99.0.0.0"},
		{Brand: "Google Chrome", Version: fullVersion},
		{Brand: "Chromium", Version: fullVersion},
	}
	return &proto.NetworkSetUserAgentOverride{
		UserAgent:      ua,
		AcceptLanguage: "en-US,en;q=0.9",
		Platform:       navPlatform,
		UserAgentMetadata: &proto.EmulationUserAgentMetadata{
			Brands:          brands,
			FullVersionList: full,
			FullVersion:     fullVersion,
			Platform:        platform,
			Architecture:    arch,
			Bitness:         "64",
		},
	}
}
