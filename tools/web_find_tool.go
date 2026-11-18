package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"pedro/tools/web/pageindex"
	"pedro/tools/web/weburl"
)

const maxHitChars = 1500

// WebFindTool searches inside the pages the agent has already read
// (with webclaw, browser or site_search) using a local full-text index.
type WebFindTool struct {
	index pageindex.Index
}

func NewWebFindTool(index pageindex.Index) *WebFindTool {
	return &WebFindTool{index: index}
}

func (WebFindTool) Definition() Definition {
	return Definition{
		Name:         "web_find",
		Description:  "Search inside web pages that were already read with webclaw, browser or site_search. Use it to find the relevant passage of a long page (e.g. a law article) instead of reading the whole page; each passage includes the offset to continue reading there with webclaw. Optionally restrict to one page (url) or one website (site).",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Words to look for",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "Only search this page",
				},
				"site": map[string]any{
					"type":        "string",
					"description": "Only search pages of this host, e.g. fedlex.admin.ch",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Number of passages to return (default 6)",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (t WebFindTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Query      string `json:"query"`
		URL        string `json:"url"`
		Site       string `json:"site"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	limit := args.MaxResults
	if limit <= 0 {
		limit = 6
	}
	hits, err := t.index.Search(pageindex.Query{Text: args.Query, URL: args.URL, Host: siteHost(args.Site), Limit: min(limit, 20)})
	if err != nil {
		return fmt.Sprintf("Search error: %v", err), nil
	}
	if len(hits) == 0 {
		scope := "the pages read so far"
		if args.URL != "" {
			scope = args.URL + " (read it with webclaw first if you have not)"
		} else if args.Site != "" {
			scope = "pages of " + args.Site + " read so far (use site_search to read the site)"
		}
		return fmt.Sprintf("No passages matching %q in %s.", args.Query, scope), nil
	}
	return formatHits(fmt.Sprintf("%d passages matching %q:", len(hits), args.Query), hits), nil
}

func formatHits(header string, hits []pageindex.Hit) string {
	var sb strings.Builder
	sb.WriteString(header + "\n\n")
	for i, h := range hits {
		label := h.Title
		if h.Heading != "" {
			if label != "" {
				label += " › "
			}
			label += h.Heading
		}
		text, _ := sliceText(h.Text, 0, maxHitChars)
		fmt.Fprintf(&sb, "[%d] %s\nURL: %s (to read on from here: webclaw offset=%d)\n%s\n\n", i+1, label, h.URL, h.Offset, text)
	}
	return strings.TrimSpace(sb.String())
}

// siteHost accepts a bare host or a URL and returns its host.
func siteHost(site string) string {
	site = strings.TrimSpace(site)
	if site == "" {
		return ""
	}
	if !strings.Contains(site, "://") {
		site = "https://" + site
	}
	return weburl.Host(site)
}
