package extract

import (
	"strings"

	"pedro/tools/webclaw"
)

// Webclaw adapts Pedro's own Readability-style extractor (with Reddit thread
// support) to the Extractor interface.
type Webclaw struct{}

func NewWebclaw() *Webclaw { return &Webclaw{} }

func (Webclaw) Name() string { return "webclaw" }

func (Webclaw) Extract(html, pageURL string) (*Article, error) {
	res, err := webclaw.Extract(html, &pageURL, nil)
	if err != nil {
		return nil, err
	}
	a := &Article{
		Markdown:  res.Content.Markdown,
		Extractor: "webclaw",
	}
	if t := res.Metadata.Title; t != nil {
		a.Title = *t
	}

	// webclaw keeps links separately; append them so navigation targets survive.
	if len(res.Content.Links) > 0 {
		var sb strings.Builder
		sb.WriteString(a.Markdown)
		sb.WriteString("\n\n## Links\n\n")
		for _, l := range res.Content.Links {
			if l.Href != "" {
				sb.WriteString("- [" + l.Text + "](" + l.Href + ")\n")
			}
		}
		a.Markdown = strings.TrimSpace(sb.String())
	}
	return a, nil
}
