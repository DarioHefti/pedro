package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// SearchTool searches the web via DuckDuckGo (falls back to Brave).
// No API key required.
type SearchTool struct{}

func NewSearchTool() *SearchTool { return &SearchTool{} }

func (SearchTool) Definition() Definition {
	return Definition{
		Name:        "web_search",
		Description: "Search the web for current information, news, and facts.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (s SearchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	results, err := s.search(args.Query)
	if err != nil {
		return fmt.Sprintf("Search error: %v", err), nil
	}

	var sb strings.Builder
	for _, r := range results {
		fmt.Fprintf(&sb, "Title: %s\nURL: %s\nSnippet: %s\n\n", r.title, r.url, r.snippet)
	}
	return sb.String(), nil
}

func (s SearchTool) search(query string) ([]searchResult, error) {
	results, err := searchDuckDuckGo(query)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	braveResults, braveErr := searchBrave(query)
	if braveErr == nil && len(braveResults) > 0 {
		return braveResults, nil
	}
	return []searchResult{{
		title:   "No results found",
		snippet: "Both DuckDuckGo and Brave returned no results. Try a different query.",
	}}, nil
}

// ── internal result type ──────────────────────────────────────────────────────

type searchResult struct {
	title   string
	url     string
	snippet string
}

// ── DuckDuckGo ────────────────────────────────────────────────────────────────

func searchDuckDuckGo(query string) ([]searchResult, error) {
	data := url.Values{"q": {query}}
	req, err := http.NewRequest("POST", "https://html.duckduckgo.com/html/", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []searchResult
	walkNodes(doc, func(n *html.Node) bool {
		if len(results) >= 5 {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "div" {
			return true
		}
		cls := getAttr(n, "class")
		if !strings.Contains(cls, "result") || strings.Contains(cls, "result--ad") {
			return true
		}

		var title, link, snippet string
		walkNodes(n, func(c *html.Node) bool {
			if c.Type != html.ElementNode || c.Data != "a" {
				return true
			}
			childCls := getAttr(c, "class")
			if hasClass(childCls, "result__a") && title == "" {
				title = textContent(c)
				link = getAttr(c, "href")
			}
			if hasClass(childCls, "result__snippet") && snippet == "" {
				snippet = textContent(c)
			}
			return true
		})

		if title != "" && link != "" {
			results = append(results, searchResult{title: title, url: link, snippet: snippet})
			return false
		}
		return true
	})

	return results, nil
}

// ── Brave ─────────────────────────────────────────────────────────────────────

func searchBrave(query string) ([]searchResult, error) {
	u := "https://search.brave.com/search?q=" + url.QueryEscape(query) + "&source=web"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var resultsDiv *html.Node
	walkNodes(doc, func(n *html.Node) bool {
		if resultsDiv != nil {
			return false
		}
		if n.Type == html.ElementNode && getAttr(n, "id") == "results" {
			resultsDiv = n
			return false
		}
		return true
	})
	if resultsDiv == nil {
		return nil, nil
	}

	var results []searchResult
	walkNodes(resultsDiv, func(n *html.Node) bool {
		if len(results) >= 5 {
			return false
		}
		if n.Type != html.ElementNode || !hasClass(getAttr(n, "class"), "snippet") {
			return true
		}

		var title, link, description string
		walkNodes(n, func(c *html.Node) bool {
			if c.Type != html.ElementNode || !hasClass(getAttr(c, "class"), "result-content") {
				return true
			}
			for child := c.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && child.Data == "a" && link == "" {
					link = getAttr(child, "href")
					walkNodes(child, func(cc *html.Node) bool {
						if cc.Type == html.ElementNode && hasClass(getAttr(cc, "class"), "search-snippet-title") {
							title = textContent(cc)
							return false
						}
						return true
					})
				}
			}
			walkNodes(c, func(cc *html.Node) bool {
				if cc.Type == html.ElementNode && hasClass(getAttr(cc, "class"), "generic-snippet") {
					description = textContent(cc)
					return false
				}
				return true
			})
			return false
		})

		if title != "" && link != "" {
			results = append(results, searchResult{title: title, url: link, snippet: description})
			return false
		}
		return true
	})

	return results, nil
}

// ── HTML helpers (used by search parsers) ────────────────────────────────────

func walkNodes(n *html.Node, fn func(*html.Node) bool) {
	if fn(n) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkNodes(c, fn)
		}
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(classes, cls string) bool {
	for _, c := range strings.Fields(classes) {
		if c == cls {
			return true
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	walkNodes(n, func(c *html.Node) bool {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		return true
	})
	return strings.TrimSpace(sb.String())
}
