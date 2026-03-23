package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const maxFetchChars = 30000

// botChallengeSignals are substrings that indicate the server blocked the request.
var botChallengeSignals = []string{
	"captcha",
	"verify you are human",
	"access denied",
	"please enable javascript",
	"enable javascript and cookies",
	"just a moment",         // Cloudflare
	"checking your browser", // Cloudflare
	"ddos protection",
	"rate limit",
	"too many requests",
	"please verify",
}

// jsRequiredSignals indicate the page needs JavaScript to render content
var jsRequiredSignals = []string{
	"javascript is required",
	"javascript muss aktiviert",   // German
	"javascript doit être activé", // French
	"javascript deve essere",      // Italian
	"enable javascript",
	"requires javascript",
	"you need to enable javascript",
	"this site requires javascript",
	"noscript",
	"<app-root></app-root>",   // Angular empty root
	"<div id=\"root\"></div>", // React empty root
	"<div id=\"app\"></div>",  // Vue empty root
}

// FetchURLTool fetches a URL and returns the content as Markdown (HTML) or
// plain text. Mirrors opencode's webfetch logic:
//   - HTML → Markdown via html-to-markdown (preserves structure for the LLM)
//   - Cloudflare cf-mitigated challenge → retry with honest User-Agent
//   - Explicit messages for bot-blocked and JS-rendered pages
type FetchURLTool struct{}

func NewFetchURLTool() *FetchURLTool { return &FetchURLTool{} }

func (FetchURLTool) Definition() Definition {
	return Definition{
		Name:        "fetch_url",
		Description: "Fetch and read the text content of a web page as Markdown. Automatically handles JavaScript-rendered pages (e.g. fedlex.admin.ch, SPAs) using a headless browser. Always try this tool first before assuming a page cannot be read. Do not fetch homepages.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The full URL to fetch",
				},
			},
			"required": []string{"url"},
		},
	}
}

func (f FetchURLTool) Execute(argsJSON string) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	return f.fetch(args.URL)
}

func (FetchURLTool) fetch(rawURL string) (string, error) {
	// Check if URL is known to require JS rendering - skip plain HTTP fetch
	if needsJSRendering(rawURL) {
		return fetchWithHeadlessBrowser(rawURL)
	}

	body, contentType, err := doFetch(rawURL)
	if err != nil {
		return fmt.Sprintf("Fetch error: %v", err), nil
	}
	raw := string(body)

	if looksLikeBlockedPage(raw) {
		// Try headless browser for blocked pages
		return fetchWithHeadlessBrowser(rawURL)
	}

	if looksLikeJSRequired(raw) {
		// Try headless browser for JS-required pages
		return fetchWithHeadlessBrowser(rawURL)
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || looksLikeHTML(raw) {
		result, err := convertToMarkdown(raw)
		if err != nil || len(strings.TrimSpace(result)) < 100 {
			// Fallback to headless browser for JS-rendered pages
			return fetchWithHeadlessBrowser(rawURL)
		}
		if len(result) > maxFetchChars {
			result = result[:maxFetchChars] + "\n\n...(truncated)"
		}
		return result, nil
	}

	// Plain text / JSON / Markdown — return as-is
	if len(raw) > maxFetchChars {
		raw = raw[:maxFetchChars] + "\n\n...(truncated)"
	}
	return raw, nil
}

// doFetch performs the HTTP GET with browser-like headers.
// If Cloudflare responds with a TLS-fingerprint challenge (cf-mitigated: challenge),
// it retries once with an honest User-Agent — the same strategy opencode uses.
func doFetch(rawURL string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	applyFetchHeaders(req, UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}

	// Cloudflare TLS-fingerprint challenge → retry with honest UA
	if resp.StatusCode == 403 && resp.Header.Get("cf-mitigated") == "challenge" {
		resp.Body.Close()
		req2, _ := http.NewRequest("GET", rawURL, nil)
		applyFetchHeaders(req2, "pedro")
		resp, err = HTTPClient.Do(req2)
		if err != nil {
			return nil, "", err
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func applyFetchHeaders(req *http.Request, ua string) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func looksLikeBlockedPage(body string) bool {
	lower := strings.ToLower(body)
	for _, s := range botChallengeSignals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func looksLikeJSRequired(body string) bool {
	lower := strings.ToLower(body)
	for _, s := range jsRequiredSignals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// needsJSRendering checks if a URL is known to require JS rendering
func needsJSRendering(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, hint := range jsRenderingHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func looksLikeHTML(s string) bool {
	p := strings.TrimSpace(s)
	if len(p) > 512 {
		p = p[:512]
	}
	p = strings.ToLower(p)
	return strings.HasPrefix(p, "<!doctype html") ||
		strings.HasPrefix(p, "<html") ||
		strings.Contains(p, "<body")
}

// convertToMarkdown converts HTML to Markdown using html-to-markdown — the Go
// equivalent of opencode's TurndownService. Noise elements are stripped first.
func convertToMarkdown(htmlContent string) (string, error) {
	converter := md.NewConverter("", true, nil)
	converter.Remove("script", "style", "noscript", "nav", "header",
		"footer", "aside", "iframe", "svg", "canvas")
	return converter.ConvertString(htmlContent)
}

// fetchWithHeadlessBrowser uses Rod (headless Chrome) to render JS-heavy pages.
// Rod auto-downloads Chromium on first use (%APPDATA%\rod\browser on Windows, ~/.cache/rod/browser on Unix).
func fetchWithHeadlessBrowser(rawURL string) (string, error) {
	// Check if a browser is available
	path, exists := launcher.LookPath()
	if !exists {
		// No Chrome/Chromium found - need to download
		fmt.Println("[Info] No Chrome/Chromium found. Downloading embedded browser (~150MB, one-time download)...")
		fmt.Println("[Info] This may take a few minutes depending on your connection...")

		b := launcher.NewBrowser()
		var err error
		path, err = b.Get()
		if err != nil {
			return fmt.Sprintf("[Failed to download browser: %v. Please install Chrome/Chromium manually, or check if antivirus is blocking the download.]", err), nil
		}
		fmt.Println("[Info] Browser download complete!")
	}

	// Launch browser (either existing or freshly downloaded)
	l := launcher.New().Bin(path).Leakless(false)
	u, err := l.Launch()
	if err != nil {
		return fmt.Sprintf("[Failed to launch browser: %v]", err), nil
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	return fetchPageContent(browser, rawURL)
}

// fetchPageContent extracts and converts page content using an existing browser instance
func fetchPageContent(browser *rod.Browser, rawURL string) (string, error) {
	page := browser.MustPage(rawURL)
	defer page.MustClose()

	// Wait for page to be fully loaded (network idle)
	err := page.WaitLoad()
	if err != nil {
		return fmt.Sprintf("[Headless browser error: %v]", err), nil
	}

	// Wait a bit more for JS to render dynamic content
	time.Sleep(2 * time.Second)

	// Wait for body to be present
	page.MustWaitStable()

	// Get the rendered HTML
	html, err := page.Evaluate(rod.Eval(`() => document.documentElement.outerHTML`))
	if err != nil {
		return fmt.Sprintf("[Could not extract page content: %v]", err), nil
	}

	htmlStr := html.Value.String()
	if htmlStr == "" {
		return "[Headless browser returned empty content]", nil
	}

	// Convert to markdown
	result, err := convertToMarkdown(htmlStr)
	if err != nil {
		return fmt.Sprintf("[Markdown conversion error: %v]", err), nil
	}

	if len(strings.TrimSpace(result)) < 50 {
		return "[Page content too short or empty after JS rendering]", nil
	}

	if len(result) > maxFetchChars {
		result = result[:maxFetchChars] + "\n\n...(truncated)"
	}

	return result, nil
}

// FetchWithJS explicitly fetches a URL using headless browser (for JS-rendered sites).
// Useful when you know in advance the page needs JS.
func FetchWithJS(rawURL string) (string, error) {
	return fetchWithHeadlessBrowser(rawURL)
}

// jsRenderingHints are domain patterns that are known to require JS rendering
var jsRenderingHints = []string{
	"fedlex.admin.ch",
}
