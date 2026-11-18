package search

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"pedro/tools/web/browser"
	"pedro/tools/web/fetch"

	"github.com/PuerkitoBio/goquery"
)

// htmlEngine scrapes a public HTML results page. It first uses the
// Chrome-impersonating HTTP client; when the engine answers with a bot
// challenge it renders the page in the stealth browser instead, and keeps
// preferring the browser for a while to avoid a doomed HTTP round trip.
type htmlEngine struct {
	name          string
	buildURL      func(query string) string
	parse         func(doc *goquery.Document) []Result
	readySelector string // CSS selector that appears once results are rendered

	http     fetch.Doer
	renderer browser.Renderer // optional

	mu                  sync.Mutex
	preferBrowserUntil  time.Time
	preferBrowserPeriod time.Duration
}

func newHTMLEngine(name string, buildURL func(string) string, parse func(*goquery.Document) []Result, readySelector string, http fetch.Doer, renderer browser.Renderer) *htmlEngine {
	return &htmlEngine{
		name:                name,
		buildURL:            buildURL,
		parse:               parse,
		readySelector:       readySelector,
		http:                http,
		renderer:            renderer,
		preferBrowserPeriod: 30 * time.Minute,
	}
}

func (e *htmlEngine) Name() string { return e.name }

func (e *htmlEngine) Search(ctx context.Context, query string) ([]Result, error) {
	u := e.buildURL(query)

	if !e.prefersBrowser() {
		results, err := e.viaHTTP(ctx, u)
		if err == nil {
			return results, nil
		}
		if e.renderer == nil || ctx.Err() != nil {
			return nil, err
		}
		e.preferBrowser()
	}
	return e.viaBrowser(ctx, u)
}

func (e *htmlEngine) viaHTTP(ctx context.Context, u string) ([]Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fetch.Do(e.http, req, 4<<20)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w (HTTP %d)", ErrBlocked, resp.StatusCode)
	}
	return e.parseHTML(string(resp.Body))
}

func (e *htmlEngine) viaBrowser(ctx context.Context, u string) ([]Result, error) {
	page, err := e.renderer.Render(ctx, u, browser.RenderOptions{ReadySelector: e.readySelector})
	if err != nil {
		return nil, err
	}
	return e.parseHTML(page.HTML)
}

// parseHTML returns the results of a page. A page without results is only
// an error when it is a bot check; otherwise the query simply had no hits.
func (e *htmlEngine) parseHTML(html string) ([]Result, error) {
	doc, err := parseDocument(html)
	if err != nil {
		return nil, err
	}
	if results := e.parse(doc); len(results) > 0 {
		return results, nil
	}
	if looksLikeBotCheck(doc) {
		return nil, ErrBlocked
	}
	return nil, nil
}

// parseDocument parses a results page without its style and script
// elements: some engines (Startpage) put CSS-in-JS <style> tags inside
// result titles, which would end up in the title text.
func parseDocument(html string) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	doc.Find("style, script, noscript").Remove()
	return doc, nil
}

// botCheckMarkers appear in the ids, classes or iframe sources of captcha and
// challenge pages (DuckDuckGo calls its bot check an "anomaly").
var botCheckMarkers = []string{"captcha", "anomaly", "challenge"}

// looksLikeBotCheck reports whether a page is a captcha or challenge rather
// than a (possibly empty) list of results.
func looksLikeBotCheck(doc *goquery.Document) bool {
	title := strings.ToLower(doc.Find("title").First().Text())
	if browser.IsChallengeTitle(title) || strings.Contains(title, "captcha") {
		return true
	}
	found := false
	doc.Find("[id], [class], iframe[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"id", "class", "src"} {
			v, _ := s.Attr(attr)
			v = strings.ToLower(v)
			for _, marker := range botCheckMarkers {
				if strings.Contains(v, marker) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

func (e *htmlEngine) prefersBrowser() bool {
	if e.renderer == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Now().Before(e.preferBrowserUntil)
}

func (e *htmlEngine) preferBrowser() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.preferBrowserUntil = time.Now().Add(e.preferBrowserPeriod)
}

// cleanText collapses whitespace in text extracted from result markup.
func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
