package search

import (
	"net/url"
	"strings"

	"pedro/tools/web/browser"
	"pedro/tools/web/fetch"

	"github.com/PuerkitoBio/goquery"
)

const maxResultsPerEngine = 10

// NewDuckDuckGo scrapes DuckDuckGo's JavaScript-free HTML endpoint.
func NewDuckDuckGo(http fetch.Doer, renderer browser.Renderer) Provider {
	return newHTMLEngine("duckduckgo",
		func(q string) string { return "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(q) },
		parseDuckDuckGo, "div.result", http, renderer)
}

// NewBrave scrapes search.brave.com (independent index).
func NewBrave(http fetch.Doer, renderer browser.Renderer) Provider {
	return newHTMLEngine("brave",
		func(q string) string { return "https://search.brave.com/search?source=web&q=" + url.QueryEscape(q) },
		parseBrave, `div.snippet[data-type="web"]`, http, renderer)
}

// NewStartpage scrapes Startpage, which serves Google results anonymously.
func NewStartpage(http fetch.Doer, renderer browser.Renderer) Provider {
	return newHTMLEngine("startpage",
		func(q string) string { return "https://www.startpage.com/sp/search?query=" + url.QueryEscape(q) },
		parseStartpage, "div.result a.result-title", http, renderer)
}

// NewMojeek scrapes Mojeek (independent index).
func NewMojeek(http fetch.Doer, renderer browser.Renderer) Provider {
	return newHTMLEngine("mojeek",
		func(q string) string { return "https://www.mojeek.com/search?q=" + url.QueryEscape(q) },
		parseMojeek, "ul.results-standard", http, renderer)
}

func parseDuckDuckGo(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div.result").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if s.HasClass("result--ad") {
			return true
		}
		a := s.Find("a.result__a").First()
		href, _ := a.Attr("href")
		link := extractDDGURL(href)
		if strings.Contains(link, "duckduckgo.com/y.js") {
			return true // sponsored link
		}
		results = appendResult(results, a.Text(), link, s.Find(".result__snippet").First().Text())
		return len(results) < maxResultsPerEngine
	})
	return results
}

func parseBrave(doc *goquery.Document) []Result {
	var results []Result
	doc.Find(`div.snippet[data-type="web"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		a := s.Find(".result-content a[href]").First()
		href, _ := a.Attr("href")
		title := a.Find(".search-snippet-title").First().Text()
		if strings.TrimSpace(title) == "" {
			title, _ = a.Find("[title]").First().Attr("title")
		}
		snippet := s.Find(".generic-snippet .content").First().Text()
		if strings.TrimSpace(snippet) == "" {
			snippet = s.Find(".generic-snippet").First().Text()
		}
		results = appendResult(results, title, href, snippet)
		return len(results) < maxResultsPerEngine
	})
	return results
}

func parseStartpage(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("div.result").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		a := s.Find("a.result-title").First()
		href, _ := a.Attr("href")
		results = appendResult(results, a.Text(), href, s.Find("p.description").First().Text())
		return len(results) < maxResultsPerEngine
	})
	return results
}

func parseMojeek(doc *goquery.Document) []Result {
	var results []Result
	doc.Find("ul.results-standard > li").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		a := s.Find("a.title").First()
		if a.Length() == 0 {
			a = s.Find("h2 a").First()
		}
		href, _ := a.Attr("href")
		results = appendResult(results, a.Text(), href, s.Find("p.s").First().Text())
		return len(results) < maxResultsPerEngine
	})
	return results
}

// appendResult adds a result if it has a title and an absolute HTTP(S) link.
func appendResult(results []Result, title, link, snippet string) []Result {
	title = cleanText(title)
	link = strings.TrimSpace(link)
	if title == "" || !(strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://")) {
		return results
	}
	return append(results, Result{Title: title, URL: link, Snippet: cleanText(snippet)})
}

// extractDDGURL extracts the destination from a DuckDuckGo redirect link
// (//duckduckgo.com/l/?uddg=<encoded-url>&rut=...).
func extractDDGURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	u := rawURL
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return rawURL
	}
	if !strings.Contains(parsed.Host, "duckduckgo.com") || !strings.HasSuffix(parsed.Path, "/l/") {
		return rawURL
	}
	if uddg := parsed.Query().Get("uddg"); uddg != "" {
		return uddg
	}
	return rawURL
}
