package extract

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// NewMarkdownConverter returns an html-to-markdown v2 converter with GitHub
// style tables and strikethrough. Converters are safe for concurrent use.
func NewMarkdownConverter() *converter.Converter {
	return converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(
				table.WithCellPaddingBehavior(table.CellPaddingBehaviorMinimal),
				table.WithSkipEmptyRows(true),
			),
			strikethrough.NewStrikethroughPlugin(),
		),
	)
}

var defaultConverter = NewMarkdownConverter()

// NodeToMarkdown converts an HTML tree to Markdown. Relative links and images
// are resolved against pageURL. The tree is modified in place.
func NodeToMarkdown(node *html.Node, pageURL string) (string, error) {
	absolutizeLinks(node, pageURL)
	md, err := defaultConverter.ConvertNode(node)
	if err != nil {
		return "", err
	}
	return tidyMarkdown(string(md)), nil
}

// absolutizeLinks resolves relative href and src attributes against pageURL
// the way a browser does ("timers" on /tickers is /timers, not /tickers/timers).
func absolutizeLinks(node *html.Node, pageURL string) {
	base, err := url.Parse(pageURL)
	if err != nil || !base.IsAbs() {
		return
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, a := range n.Attr {
				if a.Key != "href" && a.Key != "src" {
					continue
				}
				if ref, err := url.Parse(strings.TrimSpace(a.Val)); err == nil && !ref.IsAbs() {
					n.Attr[i].Val = base.ResolveReference(ref).String()
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
}

var blankLines = regexp.MustCompile(`\n{3,}`)

// tidyMarkdown strips trailing whitespace and collapses runs of blank lines,
// which some extractors emit in large amounts.
func tidyMarkdown(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\u00a0")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
