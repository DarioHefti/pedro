package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pedro/tools/webclaw"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const maxWebclawChars = 30000
const webclawRenderWait = 1200 * time.Millisecond
const webclawOpTimeout = 20 * time.Second

// WebclawTool fetches a URL and extracts structured content using webclaw
// (Readability-style scoring, noise stripping, Reddit thread extraction).
// Falls back to a headless browser for JS-rendered or bot-blocked pages.
type WebclawTool struct{}

func NewWebclawTool() *WebclawTool { return &WebclawTool{} }

func (WebclawTool) Definition() Definition {
	return Definition{
		Name:         "webclaw",
		Description:  "Fetch and read the text content of a web page as Markdown. Uses intelligent content extraction (Readability-style scoring, noise stripping, Reddit thread parsing). Automatically handles JavaScript-rendered pages with a headless browser. Always try this tool first before assuming a page cannot be read. Do not fetch homepages.",
		DeferLoading: true,
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

func (w WebclawTool) Execute(argsJSON string) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	return w.fetch(args.URL)
}

func (w WebclawTool) fetch(rawURL string) (string, error) {
	if needsJSRendering(rawURL) {
		return webclawFetchWithBrowser(rawURL)
	}

	body, contentType, err := doWebclawFetch(rawURL)
	if err != nil {
		return fmt.Sprintf("Fetch error: %v", err), nil
	}
	raw := string(body)

	if looksLikeBlockedPage(raw) {
		return webclawFetchWithBrowser(rawURL)
	}

	if looksLikeJSRequired(raw) {
		return webclawFetchWithBrowser(rawURL)
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || looksLikeHTML(raw) {
		result, err := extractWithWebclaw(raw, rawURL)
		if err != nil || len(strings.TrimSpace(result)) < 100 {
			return webclawFetchWithBrowser(rawURL)
		}
		if len(result) > maxWebclawChars {
			result = result[:maxWebclawChars] + "\n\n...(truncated)"
		}
		return result, nil
	}

	if len(raw) > maxWebclawChars {
		raw = raw[:maxWebclawChars] + "\n\n...(truncated)"
	}
	return raw, nil
}

// extractWithWebclaw runs webclaw.Extract on raw HTML and returns formatted output.
func extractWithWebclaw(htmlStr string, rawURL string) (string, error) {
	result, err := webclaw.Extract(htmlStr, &rawURL, nil)
	if err != nil {
		return "", err
	}
	return formatWebclawResult(result), nil
}

// formatWebclawResult turns an ExtractionResult into a readable string for the LLM.
func formatWebclawResult(r *webclaw.ExtractionResult) string {
	var sb strings.Builder

	if r.Metadata.Title != nil && *r.Metadata.Title != "" {
		sb.WriteString("# " + *r.Metadata.Title + "\n\n")
	}

	sb.WriteString(r.Content.Markdown)

	if len(r.Content.Links) > 0 {
		sb.WriteString("\n\n---\n\n## Links\n\n")
		for _, link := range r.Content.Links {
			if link.Href != "" {
				sb.WriteString(fmt.Sprintf("- [%s](%s)\n", link.Text, link.Href))
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// ── HTTP fetch (browser-like headers, Cloudflare retry) ─────────────────────

func doWebclawFetch(rawURL string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	applyWebclawFetchHeaders(req, UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode == 403 && resp.Header.Get("cf-mitigated") == "challenge" {
		resp.Body.Close()
		req2, _ := http.NewRequest("GET", rawURL, nil)
		applyWebclawFetchHeaders(req2, "pedro")
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

func applyWebclawFetchHeaders(req *http.Request, ua string) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

// ── Headless browser fallback ────────────────────────────────────────────────

func webclawFetchWithBrowser(rawURL string) (string, error) {
	path, exists := launcher.LookPath()
	if !exists {
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

	l := launcher.New().Bin(path).Leakless(false)
	u, err := l.Launch()
	if err != nil {
		return fmt.Sprintf("[Failed to launch browser: %v]", err), nil
	}

	browser := rod.New().ControlURL(u).MustConnect().Timeout(webclawOpTimeout)
	defer browser.MustClose()

	page := browser.MustPage(rawURL).Timeout(webclawOpTimeout)
	defer page.MustClose()

	err = page.WaitLoad()
	if err != nil {
		return fmt.Sprintf("[Headless browser error: %v]", err), nil
	}

	time.Sleep(webclawRenderWait)

	html, err := page.Evaluate(rod.Eval(`() => document.documentElement.outerHTML`))
	if err != nil {
		return fmt.Sprintf("[Could not extract page content: %v]", err), nil
	}

	htmlStr := html.Value.String()
	if htmlStr == "" {
		return "[Headless browser returned empty content]", nil
	}

	result, err := extractWithWebclaw(htmlStr, rawURL)
	if err != nil {
		return fmt.Sprintf("[Webclaw extraction error: %v]", err), nil
	}

	if len(strings.TrimSpace(result)) < 50 {
		return "[Page content too short or empty after JS rendering]", nil
	}

	if len(result) > maxWebclawChars {
		result = result[:maxWebclawChars] + "\n\n...(truncated)"
	}

	return result, nil
}
