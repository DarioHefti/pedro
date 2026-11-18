package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pedro/tools/web/search"
)

// SearchTool searches the web through Pedro's local metasearch engine
// (DuckDuckGo, Brave, Startpage, Mojeek, Wikipedia; OpenAlex for scholarly
// works). No API key or cloud search provider is involved.
type SearchTool struct {
	web       search.Searcher
	scholarly search.Searcher
}

func NewSearchTool(web, scholarly search.Searcher) *SearchTool {
	return &SearchTool{web: web, scholarly: scholarly}
}

const (
	defaultSearchResults = 8
	maxSearchResults     = 20
)

func (SearchTool) Definition() Definition {
	return Definition{
		Name:         "web_search",
		Description:  "Search the web for current information, news, and facts. Queries several search engines at once and merges their results. Set type=scholarly to search academic papers.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Number of results to return (default %d, max %d)", defaultSearchResults, maxSearchResults),
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"web", "scholarly"},
					"description": "web (default) or scholarly for academic papers",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (s SearchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
		Type       string `json:"type"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "Error: query is required", nil
	}
	limit := args.MaxResults
	if limit <= 0 {
		limit = defaultSearchResults
	}
	limit = min(limit, maxSearchResults)

	searcher := s.web
	if args.Type == "scholarly" {
		searcher = s.scholarly
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results, statuses := searcher.Search(ctx, query, limit)
	return formatSearchResults(query, results, statuses), nil
}

func formatSearchResults(query string, results []search.Result, statuses []search.EngineStatus) string {
	var sb strings.Builder
	if len(results) == 0 {
		fmt.Fprintf(&sb, "No results found for %q. Try a different query.\n", query)
	} else {
		fmt.Fprintf(&sb, "Search results for %q:\n\n", query)
	}
	for i, r := range results {
		fmt.Fprintf(&sb, "%d. Title: %s\nURL: %s\n", i+1, r.Title, r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&sb, "Snippet: %s\n", r.Snippet)
		}
		fmt.Fprintf(&sb, "Found by: %s\n\n", strings.Join(r.Engines, ", "))
	}
	sb.WriteString("Engines: " + engineSummary(statuses))
	if len(results) > 0 {
		sb.WriteString("\n\nSnippets are short and can be outdated. Read the most relevant pages with webclaw before answering.")
	}
	return sb.String()
}

func engineSummary(statuses []search.EngineStatus) string {
	parts := make([]string, 0, len(statuses))
	for _, st := range statuses {
		switch {
		case st.Skipped:
			parts = append(parts, st.Name+" (paused after recent errors)")
		case st.Err != nil:
			parts = append(parts, st.Name+" (unavailable)")
		default:
			parts = append(parts, fmt.Sprintf("%s (%d)", st.Name, st.Results))
		}
	}
	return strings.Join(parts, ", ")
}
