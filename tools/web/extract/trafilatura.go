package extract

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/markusmobius/go-trafilatura/v2"
)

// Trafilatura extracts content with go-trafilatura, which scores best on
// public extraction benchmarks and also finds author, date and language.
type Trafilatura struct{}

func NewTrafilatura() *Trafilatura { return &Trafilatura{} }

func (Trafilatura) Name() string { return "trafilatura" }

func (Trafilatura) Extract(html, pageURL string) (*Article, error) {
	opts := trafilatura.Options{
		EnableFallback:  true, // compare with the bundled readability port
		IncludeLinks:    true,
		ExcludeComments: true,
		Deduplicate:     true,
	}
	if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
		opts.OriginalURL = u
	}

	res, err := trafilatura.Extract(strings.NewReader(html), opts)
	if err != nil {
		return nil, fmt.Errorf("trafilatura: %w", err)
	}
	if res.ContentNode == nil {
		return nil, ErrNoContent
	}

	md, err := NodeToMarkdown(res.ContentNode, pageURL)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion: %w", err)
	}
	m := res.Metadata
	return &Article{
		Title:       m.Title,
		Author:      m.Author,
		SiteName:    m.Sitename,
		Description: m.Description,
		Language:    m.Language,
		Published:   m.Date,
		Markdown:    md,
		Extractor:   "trafilatura",
	}, nil
}
