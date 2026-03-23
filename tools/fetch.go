package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
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
		Description: "Fetch and read the text content of a web page as Markdown. Use this to read the full content of a specific URL found via web_search. Do not fetch homepages — they are almost always JavaScript-rendered.",
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
	body, contentType, err := doFetch(rawURL)
	if err != nil {
		return fmt.Sprintf("Fetch error: %v", err), nil
	}
	raw := string(body)

	if looksLikeBlockedPage(raw) {
		return "[This page is blocked or requires browser verification (bot challenge / CAPTCHA). Content cannot be fetched with a plain HTTP request.]", nil
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || looksLikeHTML(raw) {
		result, err := convertToMarkdown(raw)
		if err != nil || len(strings.TrimSpace(result)) < 100 {
			return "[Page appears to be JavaScript-rendered or empty. No readable content could be extracted.]", nil
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
