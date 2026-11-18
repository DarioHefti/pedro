// Package crawl reads the pages of one website, so the agent can search a
// whole site instead of guessing which page holds the answer.
package crawl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"pedro/tools/web/extract"

	"github.com/gocolly/colly/v2"
)

// Page is a crawled page.
type Page struct {
	URL     string
	Article *extract.Article
}

// Options bounds a crawl.
type Options struct {
	MaxPages int
	MaxDepth int
	Timeout  time.Duration
}

// Crawler crawls a site starting at startURL and calls visit for every page
// read. visit may be called concurrently. It returns the number of pages read.
type Crawler interface {
	Crawl(ctx context.Context, startURL string, opts Options, visit func(Page)) (int, error)
}

// Colly is a Crawler built on gocolly. It stays on the start URL's host and
// below its directory, respects robots.txt and fetches politely in parallel.
type Colly struct {
	transport http.RoundTripper
	userAgent string
	extractor extract.Extractor
}

// NewColly creates a crawler that sends requests through transport.
func NewColly(transport http.RoundTripper, userAgent string, extractor extract.Extractor) *Colly {
	return &Colly{transport: transport, userAgent: userAgent, extractor: extractor}
}

func (c *Colly) Crawl(ctx context.Context, startURL string, opts Options, visit func(Page)) (int, error) {
	start, err := url.Parse(strings.TrimSpace(startURL))
	if err != nil || (start.Scheme != "http" && start.Scheme != "https") || start.Host == "" {
		return 0, fmt.Errorf("invalid start URL %q", startURL)
	}
	start.Fragment = ""
	scope := scopeOf(start)

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	col := colly.NewCollector(
		colly.AllowedDomains(scope.hosts...),
		colly.MaxDepth(opts.MaxDepth),
		colly.MaxBodySize(5<<20),
		colly.UserAgent(c.userAgent),
		colly.StdlibContext(ctx),
		colly.Async(true),
	)
	col.IgnoreRobotsTxt = false
	col.WithTransport(c.transport)
	col.SetRequestTimeout(15 * time.Second)
	if err := col.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 4, RandomDelay: 200 * time.Millisecond}); err != nil {
		return 0, err
	}

	var mu sync.Mutex
	pages := 0
	full := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return pages >= opts.MaxPages
	}

	col.OnResponse(func(r *colly.Response) {
		if !strings.Contains(strings.ToLower(r.Headers.Get("Content-Type")), "html") {
			return
		}
		mu.Lock()
		if pages >= opts.MaxPages {
			mu.Unlock()
			return
		}
		pages++
		mu.Unlock()

		pageURL := r.Request.URL.String()
		if a, err := c.extractor.Extract(string(r.Body), pageURL); err == nil {
			visit(Page{URL: pageURL, Article: a})
		}
	})
	col.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if full() {
			return
		}
		if link, ok := scope.follow(e.Request.AbsoluteURL(e.Attr("href"))); ok {
			_ = e.Request.Visit(link)
		}
	})

	if err := col.Visit(start.String()); err != nil {
		return 0, fmt.Errorf("start crawl: %w", err)
	}
	col.Wait()

	mu.Lock()
	defer mu.Unlock()
	return pages, nil
}

type scope struct {
	hosts  []string
	prefix string
}

// scopeOf limits a crawl to the start host (with and without "www.") and to
// the directory of the start path, e.g. /docs/ for /docs/intro.
func scopeOf(start *url.URL) scope {
	host := strings.ToLower(start.Hostname())
	bare := strings.TrimPrefix(host, "www.")
	prefix := start.Path
	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix = path.Dir(prefix)
		if prefix != "/" {
			prefix += "/"
		}
	}
	return scope{hosts: []string{bare, "www." + bare}, prefix: prefix}
}

var skippedExtensions = map[string]bool{
	".pdf": true, ".zip": true, ".gz": true, ".jpg": true, ".jpeg": true, ".png": true,
	".gif": true, ".webp": true, ".svg": true, ".ico": true, ".mp3": true, ".mp4": true,
	".webm": true, ".css": true, ".js": true, ".json": true, ".xml": true, ".rss": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
}

func (s scope) follow(link string) (string, bool) {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	if !strings.HasPrefix(u.Path, s.prefix) && u.Path+"/" != s.prefix {
		return "", false
	}
	if skippedExtensions[strings.ToLower(path.Ext(u.Path))] {
		return "", false
	}
	u.Fragment = ""
	return u.String(), true
}
