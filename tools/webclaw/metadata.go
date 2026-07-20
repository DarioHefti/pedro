package webclaw

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractMetadata pulls metadata from <head>.
func extractMetadata(doc *goquery.Document) Metadata {
	meta := Metadata{
		OpenGraph: make(map[string]string),
	}

	// Title
	if title := doc.Find("title").First().Text(); title != "" {
		meta.Title = strPtr(strings.TrimSpace(title))
	}

	// Meta tags
	doc.Find("meta").Each(func(_ int, m *goquery.Selection) {
		name := m.AttrOr("name", "")
		property := m.AttrOr("property", "")
		content := m.AttrOr("content", "")
		if content == "" {
			return
		}

		switch strings.ToLower(name) {
		case "description":
			meta.Description = strPtr(content)
		case "author":
			meta.Author = strPtr(content)
		}

		if strings.HasPrefix(property, "og:") {
			key := strings.TrimPrefix(property, "og:")
			meta.OpenGraph[key] = content
		}
	})

	// Language from <html lang="...">
	doc.Find("html").Each(func(_ int, h *goquery.Selection) {
		if lang, exists := h.Attr("lang"); exists && lang != "" {
			meta.Language = strPtr(lang)
		}
	})

	return meta
}
