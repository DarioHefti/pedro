// Package extract turns raw HTML into the readable main content of a page,
// formatted as Markdown for the model.
package extract

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// Article is the readable content of a web page.
type Article struct {
	Title       string
	Author      string
	SiteName    string
	Description string
	Language    string
	Published   time.Time
	Markdown    string
	Extractor   string // name of the extractor that produced the content
}

// Extractor extracts the main content from an HTML document.
type Extractor interface {
	Name() string
	Extract(html, pageURL string) (*Article, error)
}

// ErrNoContent is returned when no readable content was found.
var ErrNoContent = errors.New("no readable content found")

// Chain tries extractors in order and returns the first result with enough
// content. If none reaches minChars, the longest result wins.
type Chain struct {
	route    func(pageURL string) []Extractor
	minChars int
}

// webclawHosts are sites where the webclaw extractor keeps structure that
// trafilatura drops: Reddit comment threads and the article headings of
// Swiss federal law on Fedlex.
var webclawHosts = []string{"reddit.com", "fedlex.admin.ch"}

// NewDefaultChain uses go-trafilatura first and the webclaw extractor as
// fallback, except for webclawHosts where the order is reversed.
func NewDefaultChain() *Chain {
	trafilatura := NewTrafilatura()
	webclaw := NewWebclaw()
	return &Chain{
		minChars: 200,
		route: func(pageURL string) []Extractor {
			if hostIn(pageURL, webclawHosts) {
				return []Extractor{webclaw, trafilatura}
			}
			return []Extractor{trafilatura, webclaw}
		},
	}
}

func (c *Chain) Name() string { return "chain" }

func (c *Chain) Extract(html, pageURL string) (*Article, error) {
	var best *Article
	var lastErr error
	for _, ex := range c.route(pageURL) {
		a, err := ex.Extract(html, pageURL)
		if err != nil {
			lastErr = err
			continue
		}
		a.Markdown = tidyMarkdown(a.Markdown)
		if len(a.Markdown) >= c.minChars {
			return a, nil
		}
		if best == nil || len(a.Markdown) > len(best.Markdown) {
			best = a
		}
	}
	if best != nil && strings.TrimSpace(best.Markdown) != "" {
		return best, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoContent
}

func hostIn(pageURL string, hosts []string) bool {
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}
