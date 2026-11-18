package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"pedro/tools/web/fetch"
)

// openAlex searches scholarly works via the free OpenAlex API.
type openAlex struct {
	http fetch.Doer
}

// NewOpenAlex returns a provider for scholarly literature.
func NewOpenAlex(http fetch.Doer) Provider {
	return &openAlex{http: http}
}

func (o *openAlex) Name() string { return "openalex" }

type openalexResponse struct {
	Results []openalexWork `json:"results"`
}

type openalexWork struct {
	ID              string `json:"id"`
	DOI             string `json:"doi"`
	DisplayName     string `json:"display_name"`
	PublicationYear int    `json:"publication_year"`
	CitedByCount    int    `json:"cited_by_count"`
	OpenAccess      struct {
		IsOA  bool   `json:"is_oa"`
		OAURL string `json:"oa_url"`
	} `json:"open_access"`
	PrimaryLocation struct {
		Source struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
	} `json:"primary_location"`
}

func (o *openAlex) Search(ctx context.Context, query string) ([]Result, error) {
	endpoint := "https://api.openalex.org/works?search=" + url.QueryEscape(query) +
		"&per-page=10&sort=relevance_score:desc&mailto=opencode@example.com"
	body, err := getJSON(ctx, o.http, endpoint)
	if err != nil {
		return nil, err
	}
	var data openalexResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse OpenAlex response: %w", err)
	}
	return openAlexResults(data.Results), nil
}

func openAlexResults(works []openalexWork) []Result {
	var results []Result
	for _, w := range works {
		if w.DisplayName == "" {
			continue
		}
		link := w.DOI
		if link == "" {
			link = w.ID
		}

		var details []string
		if w.PublicationYear > 0 {
			details = append(details, fmt.Sprintf("Published %d", w.PublicationYear))
		}
		if w.CitedByCount > 0 {
			details = append(details, fmt.Sprintf("Cited by %d", w.CitedByCount))
		}
		if src := w.PrimaryLocation.Source.DisplayName; src != "" {
			details = append(details, src)
		}
		if w.OpenAccess.IsOA {
			oa := "Open Access"
			if w.OpenAccess.OAURL != "" {
				oa += " (" + w.OpenAccess.OAURL + ")"
			}
			details = append(details, oa)
		}

		results = append(results, Result{
			Title:   "[Scholarly] " + w.DisplayName,
			URL:     link,
			Snippet: strings.Join(details, " · "),
		})
	}
	return results
}
