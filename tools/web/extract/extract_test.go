package extract

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const article = `<!DOCTYPE html><html lang="en"><head><title>Rent law basics | Example News</title>
<meta name="author" content="Jane Doe"><meta property="og:site_name" content="Example News"></head>
<body><nav><a href="/">Home</a> <a href="/news">News</a></nav>
<article><h1>Rent law basics</h1>
<p>Tenants in Switzerland can terminate a lease with three months notice, usually to one of the official dates set by the canton.</p>
<p>The notice must be given in writing and signed. For family homes both spouses must sign, otherwise the notice is void. See the <a href="/guide/notice">notice guide</a> for templates.</p>
<table><tr><th>Canton</th><th>Dates</th></tr><tr><td>Zurich</td><td>end of March and September</td></tr></table>
<p>Landlords must use the official cantonal form, and the notice period for landlords is the same as for tenants in most cases.</p>
</article><footer>Copyright Example News</footer></body></html>`

func TestNodeToMarkdownResolvesRelativeLinksLikeABrowser(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<p><a href="timers">Timers</a> <a href="../up">Up</a> <a href="/root">Root</a> <a href="#top">Top</a> <a href="mailto:a@b.ch">Mail</a></p>`))
	if err != nil {
		t.Fatal(err)
	}
	md, err := NodeToMarkdown(doc, "https://gobyexample.com/docs/tickers")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(https://gobyexample.com/docs/timers)",
		"(https://gobyexample.com/up)",
		"(https://gobyexample.com/root)",
		"(https://gobyexample.com/docs/tickers#top)",
		"(mailto:a@b.ch)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %s in %q", want, md)
		}
	}
}

func TestTrafilaturaExtractsArticleAsMarkdown(t *testing.T) {
	a, err := NewTrafilatura().Extract(article, "https://news.example.com/2026/rent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Markdown, "three months notice") {
		t.Errorf("main text missing: %q", a.Markdown)
	}
	if !strings.Contains(a.Markdown, "(https://news.example.com/guide/notice)") {
		t.Errorf("relative links should be absolute: %q", a.Markdown)
	}
	if !strings.Contains(a.Markdown, "| Zurich |") {
		t.Errorf("tables should be kept: %q", a.Markdown)
	}
	if strings.Contains(a.Markdown, "Copyright") {
		t.Errorf("boilerplate should be removed: %q", a.Markdown)
	}
	if a.Author != "Jane Doe" || a.Language != "en" || a.Extractor != "trafilatura" {
		t.Errorf("unexpected metadata %+v", a)
	}
}

type stub struct {
	name string
	md   string
	err  error
}

func (s stub) Name() string { return s.name }
func (s stub) Extract(string, string) (*Article, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &Article{Markdown: s.md, Extractor: s.name}, nil
}

func TestChainFallsBackAndPicksLongest(t *testing.T) {
	long := strings.Repeat("word ", 60)
	c := &Chain{minChars: 200, route: func(string) []Extractor {
		return []Extractor{stub{name: "a", md: "short"}, stub{name: "b", err: ErrNoContent}, stub{name: "c", md: long}}
	}}
	a, err := c.Extract("", "https://x.com")
	if err != nil || a.Extractor != "c" {
		t.Fatalf("expected c, got %+v err=%v", a, err)
	}

	c.route = func(string) []Extractor {
		return []Extractor{stub{name: "a", md: "short"}, stub{name: "b", md: "tiny"}}
	}
	if a, _ := c.Extract("", "https://x.com"); a.Extractor != "a" {
		t.Errorf("expected the longest short result, got %+v", a)
	}

	c.route = func(string) []Extractor { return []Extractor{stub{name: "a", err: ErrNoContent}} }
	if _, err := c.Extract("", "https://x.com"); err == nil {
		t.Error("expected an error when every extractor fails")
	}
}

func TestDefaultChainRoutesStructuredSitesToWebclaw(t *testing.T) {
	c := NewDefaultChain()
	for _, u := range []string{"https://old.reddit.com/r/golang/comments/x", "https://www.fedlex.admin.ch/eli/cc/27/317_321_377/de"} {
		if got := c.route(u)[0].Name(); got != "webclaw" {
			t.Errorf("%s should use webclaw first, got %s", u, got)
		}
	}
	if got := c.route("https://example.com/")[0].Name(); got != "trafilatura" {
		t.Errorf("default should be trafilatura, got %s", got)
	}
}

func TestTidyMarkdown(t *testing.T) {
	in := "a  \n\n\n\n   \n\t\nb \r\n\r\nc"
	if got := tidyMarkdown(in); got != "a\n\nb\n\nc" {
		t.Errorf("tidyMarkdown = %q", got)
	}
}
