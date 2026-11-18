package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"pedro/tools/web/crawl"
	"pedro/tools/web/pageindex"
	"pedro/tools/web/weburl"
)

// SiteSearchTool crawls a website (same host, below the start URL's folder)
// into the local page index and searches it.
type SiteSearchTool struct {
	crawler crawl.Crawler
	index   pageindex.Index

	mu      sync.Mutex
	crawled map[string]time.Time // start URL key -> time of the last crawl
}

func NewSiteSearchTool(crawler crawl.Crawler, index pageindex.Index) *SiteSearchTool {
	return &SiteSearchTool{crawler: crawler, index: index, crawled: make(map[string]time.Time)}
}

const (
	siteCrawlPages    = 30
	siteCrawlDepth    = 3
	siteCrawlTimeout  = 45 * time.Second
	siteCrawlReuseFor = 30 * time.Minute
)

func (*SiteSearchTool) Definition() Definition {
	return Definition{
		Name:         "site_search",
		Description:  fmt.Sprintf("Search within one website: reads up to %d pages of the site starting at url (staying in that folder of the site) and returns the passages that best match the query. Use it when you know which site has the answer but not which page.", siteCrawlPages),
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Where to start, e.g. https://www.example.ch/help/",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "What to look for",
				},
			},
			"required": []string{"url", "query"},
		},
	}
}

func (t *SiteSearchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		URL   string `json:"url"`
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	if strings.TrimSpace(args.Query) == "" {
		return "Error: query is required", nil
	}
	start, err := url.Parse(strings.TrimSpace(args.URL))
	if err != nil || start.Host == "" {
		return fmt.Sprintf("Error: invalid url %q", args.URL), nil
	}
	host := weburl.Host(args.URL)

	note := ""
	if !t.recentlyCrawled(args.URL) {
		ctx, cancel := context.WithTimeout(context.Background(), siteCrawlTimeout+10*time.Second)
		defer cancel()
		opts := crawl.Options{MaxPages: siteCrawlPages, MaxDepth: siteCrawlDepth, Timeout: siteCrawlTimeout}
		n, err := t.crawler.Crawl(ctx, args.URL, opts, func(p crawl.Page) {
			indexArticle(t.index, p.URL, p.Article)
		})
		if err != nil {
			return fmt.Sprintf("Crawl error: %v", err), nil
		}
		if n == 0 {
			return fmt.Sprintf("Could not read any page of %s. The site may block crawlers or need JavaScript; try the browser tool instead.", host), nil
		}
		t.markCrawled(args.URL)
		note = fmt.Sprintf("Read %d pages of %s.", n, host)
	}

	hits, err := t.index.Search(pageindex.Query{Text: args.Query, Host: host, Limit: 8})
	if err != nil {
		return fmt.Sprintf("Search error: %v", err), nil
	}
	if len(hits) == 0 {
		return strings.TrimSpace(fmt.Sprintf("%s No passages on %s match %q. Try other words, or start from a different section of the site.", note, host, args.Query)), nil
	}
	header := strings.TrimSpace(fmt.Sprintf("%s Best matches for %q on %s:", note, args.Query, host))
	return formatHits(header, hits), nil
}

func (t *SiteSearchTool) recentlyCrawled(rawURL string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	at, ok := t.crawled[weburl.Key(rawURL)]
	return ok && time.Since(at) < siteCrawlReuseFor
}

func (t *SiteSearchTool) markCrawled(rawURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.crawled[weburl.Key(rawURL)] = time.Now()
}
