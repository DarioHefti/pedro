package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"pedro/tools/web/extract"
	"pedro/tools/web/pageindex"
	"pedro/tools/web/reader"
)

const (
	maxWebclawChars = 30000
	// freshPageAge is how long a read page is served from the index instead
	// of downloading it again (the model often re-requests URL variants such
	// as page#section).
	freshPageAge = 10 * time.Minute
	// pageCacheAge is how long a read page is served from the index when the
	// model continues reading it with an offset.
	pageCacheAge = 30 * time.Minute
)

// WebclawTool reads a web page as Markdown. Pages are fetched with a
// Chrome-impersonating client, rendered in a stealth headless browser when
// they need JavaScript or show a bot check, and indexed for web_find.
type WebclawTool struct {
	reader reader.Reader
	index  pageindex.Index
}

func NewWebclawTool(r reader.Reader, index pageindex.Index) *WebclawTool {
	return &WebclawTool{reader: r, index: index}
}

func (WebclawTool) Definition() Definition {
	return Definition{
		Name:         "webclaw",
		Description:  "Fetch and read the text content of a web page (or an online PDF, Word or PowerPoint document) as Markdown. Extracts the main content, handles JavaScript-rendered and bot-protected pages with a headless browser. Long pages are returned in parts: pass offset to continue reading, or use web_find to search the page. Always try this tool first before assuming a page cannot be read. Do not fetch homepages.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The full URL to fetch",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Position to continue reading a long page from (given at the end of the previous part)",
				},
			},
			"required": []string{"url"},
		},
	}
}

func (w WebclawTool) Execute(argsJSON string) (string, error) {
	var args struct {
		URL    string `json:"url"`
		Offset int    `json:"offset"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	maxAge := freshPageAge
	if args.Offset > 0 {
		maxAge = pageCacheAge
	}
	if cached, ok := w.index.Get(args.URL, maxAge); ok {
		return formatPagePart(cached.URL, &extract.Article{Title: cached.Title, Markdown: cached.Markdown}, args.Offset), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	page, err := w.reader.Read(ctx, args.URL)
	if err != nil {
		return fmt.Sprintf("Fetch error: %v", err), nil
	}
	indexArticle(w.index, page.URL, page.Article)
	if page.Status >= 400 {
		return fmt.Sprintf("[HTTP %d]\n\n%s", page.Status, formatPagePart(page.URL, page.Article, args.Offset)), nil
	}
	return formatPagePart(page.URL, page.Article, args.Offset), nil
}

// formatPagePart renders a header with the page metadata followed by the
// part of the content starting at offset.
func formatPagePart(pageURL string, a *extract.Article, offset int) string {
	var sb strings.Builder
	if a.Title != "" {
		sb.WriteString("# " + a.Title + "\n")
	}
	sb.WriteString("URL: " + pageURL + "\n")

	var meta []string
	if a.Author != "" {
		meta = append(meta, "Author: "+a.Author)
	}
	if !a.Published.IsZero() {
		meta = append(meta, "Published: "+a.Published.Format("2006-01-02"))
	}
	if a.Language != "" {
		meta = append(meta, "Language: "+a.Language)
	}
	if len(meta) > 0 {
		sb.WriteString(strings.Join(meta, " · ") + "\n")
	}
	sb.WriteString("\n")

	part, next := sliceText(a.Markdown, offset, maxWebclawChars)
	if part == "" && offset > 0 {
		fmt.Fprintf(&sb, "[No content after offset %d; the page has %d characters.]", offset, len(a.Markdown))
		return sb.String()
	}
	sb.WriteString(part)
	if next < len(a.Markdown) {
		fmt.Fprintf(&sb, "\n\n[Showing characters %d-%d of %d. To continue reading call webclaw with offset=%d, or use web_find with url=%q to search the rest of this page.]",
			offset, next, len(a.Markdown), next, pageURL)
	}
	return sb.String()
}

// sliceText returns up to limit bytes of s starting at offset, ending on a
// line break when one is near the limit and never splitting a UTF-8 rune.
// next is the offset where the following part starts.
func sliceText(s string, offset, limit int) (part string, next int) {
	if offset < 0 {
		offset = 0
	}
	for offset < len(s) && !utf8.RuneStart(s[offset]) {
		offset++
	}
	if offset >= len(s) {
		return "", len(s)
	}
	end := offset + limit
	if end >= len(s) {
		return s[offset:], len(s)
	}
	if nl := strings.LastIndexByte(s[offset:end], '\n'); nl > limit*9/10 {
		end = offset + nl + 1
	}
	for end > offset && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[offset:end], end
}
