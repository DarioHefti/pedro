package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// SearchTool searches the web via DuckDuckGo, OpenAlex (scholarly), and Brave.
// No API key required.
type SearchTool struct{}

func NewSearchTool() *SearchTool { return &SearchTool{} }

func (SearchTool) Definition() Definition {
	return Definition{
		Name:         "web_search",
		Description:  "Search the web for current information, news, and facts.",
		DeferLoading: true,
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
	var errs []string

	results, err := searchDuckDuckGo(query)
	if err != nil {
		errs = append(errs, fmt.Sprintf("DuckDuckGo error: %v", err))
	} else if len(results) > 0 {
		return results, nil
	} else {
		errs = append(errs, "DuckDuckGo returned no results")
	}

	braveResults, braveErr := searchBrave(query)
	if braveErr != nil {
		errs = append(errs, fmt.Sprintf("Brave error: %v", braveErr))
	} else if len(braveResults) > 0 {
		return braveResults, nil
	} else {
		errs = append(errs, "Brave returned no results")
	}

	openalexResults, openalexErr := searchOpenAlex(query)
	if openalexErr != nil {
		errs = append(errs, fmt.Sprintf("OpenAlex error: %v", openalexErr))
	} else if len(openalexResults) > 0 {
		return openalexResults, nil
	} else {
		errs = append(errs, "OpenAlex returned no results")
	}

	errMsg := "No results found"
	if len(errs) > 0 {
		errMsg = fmt.Sprintf("%s. Details: %s", errMsg, strings.Join(errs, "; "))
	}
	return []searchResult{{
		title:   "No results found",
		snippet: fmt.Sprintf("%s. Try a different query.", errMsg),
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
				link = extractDDGURL(getAttr(c, "href"))
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

// ── OpenAlex (scholarly works) ────────────────────────────────────────────────

// openalexResponse represents the JSON response from the OpenAlex API.
type openalexResponse struct {
	Meta struct {
		Count int `json:"count"`
	} `json:"meta"`
	Results []openalexWork `json:"results"`
}

// openalexWork represents a single scholarly work from OpenAlex.
type openalexWork struct {
	ID               string `json:"id"`
	DOI              string `json:"doi"`
	DisplayName      string `json:"display_name"`
	PublicationYear  int    `json:"publication_year"`
	CitedByCount     int    `json:"cited_by_count"`
	OpenAccess       struct {
		IsOA    bool   `json:"is_oa"`
		OAURL   string `json:"oa_url"`
		OAStatus string `json:"oa_status"`
	} `json:"open_access"`
	PrimaryLocation struct {
		Source struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
	} `json:"primary_location"`
}

func searchOpenAlex(query string) ([]searchResult, error) {
	u := "https://api.openalex.org/works?search=" + url.QueryEscape(query) + "&per-page=5&sort=relevance_score:desc&mailto=opencode@example.com"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAlex returned status %d", resp.StatusCode)
	}

	var apiResp openalexResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAlex response: %w", err)
	}

	var results []searchResult
	for _, w := range apiResp.Results {
		title := w.DisplayName
		if title == "" {
			continue
		}

		link := w.DOI
		if link == "" {
			link = w.ID
		}

		var sb strings.Builder
		if w.PublicationYear > 0 {
			fmt.Fprintf(&sb, "Published %d", w.PublicationYear)
		}
		if w.CitedByCount > 0 {
			if sb.Len() > 0 {
				sb.WriteString(" · ")
			}
			fmt.Fprintf(&sb, "Cited by %d", w.CitedByCount)
		}
		if w.PrimaryLocation.Source.DisplayName != "" {
			if sb.Len() > 0 {
				sb.WriteString(" · ")
			}
			sb.WriteString(w.PrimaryLocation.Source.DisplayName)
		}
		if w.OpenAccess.IsOA {
			if sb.Len() > 0 {
				sb.WriteString(" · ")
			}
			sb.WriteString("Open Access")
			if w.OpenAccess.OAURL != "" {
				fmt.Fprintf(&sb, " (%s)", w.OpenAccess.OAURL)
			}
		}

		results = append(results, searchResult{
			title:   fmt.Sprintf("[Scholarly] %s", title),
			url:     link,
			snippet: sb.String(),
		})
	}

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

// extractDDGURL extracts the actual destination URL from a DuckDuckGo redirect link.
// DDG wraps URLs as //duckduckgo.com/l/?uddg=<encoded-url>&rut=...
func extractDDGURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	// Handle protocol-relative URLs
	u := rawURL
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return rawURL
	}

	// Check if this is a DDG redirect link
	if !strings.Contains(parsed.Host, "duckduckgo.com") || !strings.HasSuffix(parsed.Path, "/l/") {
		return rawURL
	}

	// Extract the uddg parameter which contains the actual URL
	uddg := parsed.Query().Get("uddg")
	if uddg != "" {
		if decoded, err := url.QueryUnescape(uddg); err == nil {
			return decoded
		}
	}

	return rawURL
}
