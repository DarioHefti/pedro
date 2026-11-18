// Package reader downloads a web page and returns its readable content,
// escalating from a Chrome-impersonating HTTP request to a stealth headless
// browser only when the page needs JavaScript or answers with a bot check.
package reader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"pedro/documentparser"
	"pedro/tools/web/browser"
	"pedro/tools/web/extract"
	"pedro/tools/web/fetch"
)

// Page is a page that was read.
type Page struct {
	URL      string // final URL after redirects
	Status   int    // HTTP status of the static fetch (0 when rendered)
	Article  *extract.Article
	Rendered bool // true when the headless browser was needed
}

// Reader reads web pages.
type Reader interface {
	Read(ctx context.Context, rawURL string) (*Page, error)
}

// Pipeline is the default Reader.
type Pipeline struct {
	http      fetch.Doer
	fallback  fetch.Doer       // optional plain client for the Cloudflare retry
	renderer  browser.Renderer // optional
	extractor extract.Extractor
	documents documentparser.Parser // optional, for PDFs and Office files
}

// New builds a Pipeline. fallback, renderer and documents may be nil.
func New(http, fallback fetch.Doer, renderer browser.Renderer, extractor extract.Extractor, documents documentparser.Parser) *Pipeline {
	return &Pipeline{http: http, fallback: fallback, renderer: renderer, extractor: extractor, documents: documents}
}

const (
	// maxBodyBytes is large enough for typical PDF reports.
	maxBodyBytes = 32 << 20
	// minUsefulChars is the amount of extracted text below which we assume the
	// static HTML was an app shell or interstitial and try the browser.
	minUsefulChars = 300
)

// ErrBotCheck is returned when a page stays behind a bot check.
var ErrBotCheck = errors.New("blocked by a bot check")

// ErrUnsupportedContent is returned for binary content such as images.
var ErrUnsupportedContent = errors.New("unsupported content type")

func (p *Pipeline) Read(ctx context.Context, rawURL string) (*Page, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: only absolute http(s) URLs are supported", rawURL)
	}
	rawURL = u.String()

	if p.renderer != nil && needsJSRendering(rawURL) {
		return p.render(ctx, rawURL, nil)
	}

	resp, err := p.download(ctx, rawURL)
	if err != nil {
		if p.renderer != nil && ctx.Err() == nil {
			return p.render(ctx, rawURL, err)
		}
		return nil, err
	}

	if ext := documentExt(resp); ext != "" && p.documents != nil {
		return p.parseDocument(resp, ext)
	}
	if !isHTML(resp) {
		return textPage(resp)
	}
	if looksBlocked(resp) && p.renderer != nil {
		return p.render(ctx, rawURL, fmt.Errorf("HTTP %d bot check", resp.StatusCode))
	}

	article, extractErr := p.extractor.Extract(string(resp.Body), resp.URL)
	page := &Page{URL: resp.URL, Status: resp.StatusCode, Article: article}
	if extractErr == nil && len(article.Markdown) >= minUsefulChars {
		return page, nil
	}
	if p.renderer != nil {
		rendered, err := p.render(ctx, rawURL, extractErr)
		switch {
		case err == nil && (extractErr != nil || len(rendered.Article.Markdown) > len(article.Markdown)):
			return rendered, nil
		case extractErr != nil:
			return nil, err
		}
	}
	if extractErr != nil {
		return nil, extractErr
	}
	return page, nil
}

func (p *Pipeline) download(ctx context.Context, rawURL string) (*fetch.Response, error) {
	resp, err := fetch.Get(ctx, p.http, rawURL, maxBodyBytes)
	if err != nil {
		return nil, err
	}
	// Cloudflare sometimes challenges browser-looking clients but lets
	// self-identified non-browser agents through.
	if p.fallback != nil && resp.StatusCode == http.StatusForbidden && resp.Header.Get("cf-mitigated") == "challenge" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "pedro")
			if retry, err := fetch.Do(p.fallback, req, maxBodyBytes); err == nil && retry.StatusCode == http.StatusOK {
				return retry, nil
			}
		}
	}
	return resp, nil
}

// render reads the page with the headless browser. cause explains why the
// static fetch was not good enough and is reported if rendering fails too.
func (p *Pipeline) render(ctx context.Context, rawURL string, cause error) (*Page, error) {
	r, err := p.renderer.Render(ctx, rawURL, browser.RenderOptions{})
	if err != nil {
		if cause != nil {
			return nil, fmt.Errorf("%v; headless browser also failed: %w", cause, err)
		}
		return nil, fmt.Errorf("headless browser: %w", err)
	}
	if browser.IsChallengeTitle(r.Title) {
		return nil, fmt.Errorf("%w: %s asks for human verification (captcha), so neither webclaw nor the browser tool can read it; use another source", ErrBotCheck, hostOf(rawURL))
	}
	article, err := p.extractor.Extract(r.HTML, r.URL)
	if err != nil {
		return nil, err
	}
	return &Page{URL: r.URL, Article: article, Rendered: true}, nil
}

// documentTypes maps document content types to the file extension the
// document parser expects.
var documentTypes = map[string]string{
	"application/pdf": ".pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.oasis.opendocument.text":                                   ".odt",
}

// documentExt returns the extension of a downloaded document, or "" for
// anything else. Servers often send documents as application/octet-stream,
// so the URL's extension and the PDF magic bytes count too.
func documentExt(resp *fetch.Response) string {
	if ext, ok := documentTypes[resp.ContentType]; ok {
		return ext
	}
	if strings.HasPrefix(string(resp.Body[:min(len(resp.Body), 5)]), "%PDF-") {
		return ".pdf"
	}
	if resp.ContentType == "application/octet-stream" {
		ext := strings.ToLower(path.Ext(urlPath(resp.URL)))
		for _, known := range documentTypes {
			if ext == known {
				return ext
			}
		}
	}
	return ""
}

// parseDocument converts a downloaded PDF or Office file to Markdown.
func (p *Pipeline) parseDocument(resp *fetch.Response, ext string) (*Page, error) {
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if len(resp.Body) >= maxBodyBytes {
		return nil, fmt.Errorf("%w: the document is larger than %d MB", ErrUnsupportedContent, maxBodyBytes>>20)
	}
	f, err := os.CreateTemp("", "pedro-web-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(resp.Body)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	doc, err := p.documents.Parse(f.Name())
	if err != nil {
		return nil, fmt.Errorf("could not read the %s document: %w", strings.TrimPrefix(ext, "."), err)
	}

	var md strings.Builder
	for _, line := range doc.Lines {
		// The PDF parser marks page starts; headings let web_find label passages by page.
		if page, ok := strings.CutPrefix(line, "--- Page "); ok {
			md.WriteString("\n## Page " + strings.TrimSuffix(page, " ---") + "\n\n")
			continue
		}
		md.WriteString(line + "\n")
	}
	text := strings.TrimSpace(md.String())
	if text == "" {
		return nil, fmt.Errorf("%w: the %s document has no extractable text (it may be scanned images)", ErrUnsupportedContent, strings.TrimPrefix(ext, "."))
	}
	return &Page{
		URL:     resp.URL,
		Status:  resp.StatusCode,
		Article: &extract.Article{Title: path.Base(urlPath(resp.URL)), Markdown: text, Extractor: doc.Format},
	}, nil
}

func urlPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

func textPage(resp *fetch.Response) (*Page, error) {
	ct := resp.ContentType
	textual := strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") || strings.Contains(ct, "javascript") || ct == ""
	if !textual {
		return nil, fmt.Errorf("%w %q: this URL is a file, not a web page", ErrUnsupportedContent, ct)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return &Page{
		URL:     resp.URL,
		Status:  resp.StatusCode,
		Article: &extract.Article{Markdown: string(resp.Body), Extractor: "raw"},
	}, nil
}

func isHTML(resp *fetch.Response) bool {
	if strings.Contains(resp.ContentType, "html") {
		return true
	}
	head := strings.ToLower(strings.TrimSpace(string(resp.Body[:min(len(resp.Body), 512)])))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html") || strings.Contains(head, "<body")
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// looksBlocked detects bot-protection interstitials of a static response.
func looksBlocked(resp *fetch.Response) bool {
	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusAccepted:
		return true
	}
	if resp.Header.Get("cf-mitigated") != "" {
		return true
	}
	head := resp.Body[:min(len(resp.Body), 64<<10)]
	if m := titleRe.FindSubmatch(head); m != nil {
		return browser.IsChallengeTitle(string(m[1]))
	}
	return false
}

// jsRenderingHosts are sites known to serve an empty shell without JavaScript.
var jsRenderingHosts = []string{
	"fedlex.admin.ch",
}

func needsJSRendering(rawURL string) bool {
	host := hostOf(rawURL)
	for _, h := range jsRenderingHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
