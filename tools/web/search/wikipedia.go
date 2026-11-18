package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"pedro/tools/web/fetch"

	"golang.org/x/net/html"
)

// wikipedia queries the public MediaWiki search API.
type wikipedia struct {
	lang string
	http fetch.Doer
}

// NewWikipedia searches <lang>.wikipedia.org.
func NewWikipedia(lang string, http fetch.Doer) Provider {
	return &wikipedia{lang: lang, http: http}
}

func (w *wikipedia) Name() string { return "wikipedia" }

func (w *wikipedia) Search(ctx context.Context, query string) ([]Result, error) {
	endpoint := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=query&list=search&format=json&utf8=1&srlimit=3&srsearch=%s",
		w.lang, url.QueryEscape(query))
	body, err := getJSON(ctx, w.http, endpoint)
	if err != nil {
		return nil, err
	}

	var data struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse Wikipedia response: %w", err)
	}

	var results []Result
	for _, s := range data.Query.Search {
		page := url.PathEscape(strings.ReplaceAll(s.Title, " ", "_"))
		results = append(results, Result{
			Title:   s.Title + " - Wikipedia",
			URL:     fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", w.lang, page),
			Snippet: stripTags(s.Snippet),
		})
	}
	return results, nil
}

// getJSON performs an API-style GET request (as a page script would) and
// returns the body of a 200 response.
func getJSON(ctx context.Context, d fetch.Doer, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := fetch.Do(d, req, 4<<20)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// stripTags turns an HTML fragment into plain text.
func stripTags(fragment string) string {
	z := html.NewTokenizer(strings.NewReader(fragment))
	var sb strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return cleanText(sb.String())
		case html.TextToken:
			sb.Write(z.Text())
		}
	}
}
