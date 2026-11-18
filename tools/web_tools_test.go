package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"pedro/tools/web/browser"
	"pedro/tools/web/crawl"
	"pedro/tools/web/extract"
	"pedro/tools/web/pageindex"
	"pedro/tools/web/reader"
	"pedro/tools/web/search"
)

func newTestIndex(t *testing.T) pageindex.Index {
	t.Helper()
	idx, err := pageindex.NewSQLiteIndex(50)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func args(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ── web_search ───────────────────────────────────────────────────────────────

type fakeSearcher struct {
	query string
	limit int
}

func (f *fakeSearcher) Search(_ context.Context, q string, limit int) ([]search.Result, []search.EngineStatus) {
	f.query, f.limit = q, limit
	return []search.Result{{Title: "Go", URL: "https://go.dev", Snippet: "The Go language", Engines: []string{"duckduckgo", "brave"}}},
		[]search.EngineStatus{{Name: "duckduckgo", Results: 10}, {Name: "mojeek", Err: search.ErrBlocked}, {Name: "brave", Skipped: true}}
}

func TestSearchToolFormatsResultsAndEngines(t *testing.T) {
	web, scholarly := &fakeSearcher{}, &fakeSearcher{}
	tool := NewSearchTool(web, scholarly)

	out, _ := tool.Execute(args(t, map[string]any{"query": " golang ", "max_results": 99}))
	for _, want := range []string{"1. Title: Go", "URL: https://go.dev", "Snippet: The Go language", "Found by: duckduckgo, brave",
		"duckduckgo (10)", "mojeek (unavailable)", "brave (paused after recent errors)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if web.query != "golang" || web.limit != maxSearchResults {
		t.Errorf("query=%q limit=%d", web.query, web.limit)
	}

	tool.Execute(args(t, map[string]any{"query": "transformers", "type": "scholarly"}))
	if scholarly.query != "transformers" || scholarly.limit != defaultSearchResults {
		t.Errorf("scholarly searcher not used: %+v", scholarly)
	}

	if out, _ := tool.Execute(`{"query":"  "}`); !strings.Contains(out, "query is required") {
		t.Errorf("empty query: %q", out)
	}
}

// ── webclaw ──────────────────────────────────────────────────────────────────

type fakeReader struct {
	page  *reader.Page
	err   error
	calls int
}

func (f *fakeReader) Read(context.Context, string) (*reader.Page, error) {
	f.calls++
	return f.page, f.err
}

func TestWebclawPagesThroughLongContentFromCache(t *testing.T) {
	long := strings.Repeat("Absatz über Kündigungsfristen im Mietrecht.\n", 1500) // ~66k bytes
	r := &fakeReader{page: &reader.Page{URL: "https://example.ch/law", Status: 200,
		Article: &extract.Article{Title: "Mietrecht", Language: "de", Markdown: long}}}
	tool := NewWebclawTool(r, newTestIndex(t))

	first, _ := tool.Execute(`{"url":"https://example.ch/law"}`)
	if !strings.HasPrefix(first, "# Mietrecht\nURL: https://example.ch/law\nLanguage: de\n") {
		t.Errorf("unexpected header: %q", first[:80])
	}
	if !strings.Contains(first, "call webclaw with offset=") || !strings.Contains(first, `web_find with url="https://example.ch/law"`) {
		t.Errorf("missing continuation hint: %q", first[len(first)-300:])
	}

	again, _ := tool.Execute(`{"url":"https://example.ch/law#section"}`)
	if r.calls != 1 || !strings.Contains(again, "Absatz über Kündigungsfristen") {
		t.Errorf("re-reading a URL variant should be served from the index (reader calls %d)", r.calls)
	}

	second, _ := tool.Execute(`{"url":"https://example.ch/law","offset":30000}`)
	if r.calls != 1 {
		t.Errorf("continuing should be served from the index, reader called %d times", r.calls)
	}
	if !strings.Contains(second, "Showing characters 30000-") {
		t.Errorf("unexpected second part footer: %q", second[len(second)-200:])
	}

	end, _ := tool.Execute(`{"url":"https://example.ch/law","offset":999999}`)
	if !strings.Contains(end, "No content after offset") {
		t.Errorf("unexpected output past the end: %q", end)
	}
}

func TestWebclawReportsErrorsAndHTTPStatus(t *testing.T) {
	tool := NewWebclawTool(&fakeReader{err: errors.New("boom")}, newTestIndex(t))
	if out, _ := tool.Execute(`{"url":"https://x.ch"}`); out != "Fetch error: boom" {
		t.Errorf("got %q", out)
	}
	tool = NewWebclawTool(&fakeReader{page: &reader.Page{URL: "https://x.ch/missing", Status: 404, Article: &extract.Article{Markdown: "Not found"}}}, newTestIndex(t))
	if out, _ := tool.Execute(`{"url":"https://x.ch/missing"}`); !strings.HasPrefix(out, "[HTTP 404]") {
		t.Errorf("got %q", out)
	}
	if out, _ := tool.Execute("{not json"); !strings.Contains(out, "Error parsing arguments") {
		t.Errorf("got %q", out)
	}
}

func TestSliceTextNeverSplitsRunes(t *testing.T) {
	s := strings.Repeat("äöü€", 100)
	for _, offset := range []int{0, 1, 2, 3, 50} {
		part, next := sliceText(s, offset, 37)
		if !utf8.ValidString(part) || next <= offset || !utf8.RuneStart(s[min(next, len(s)-1)]) {
			t.Errorf("offset %d: part=%q next=%d", offset, part, next)
		}
	}
	if part, next := sliceText("abc", 0, 10); part != "abc" || next != 3 {
		t.Errorf("short text: %q %d", part, next)
	}
}

// ── web_find ─────────────────────────────────────────────────────────────────

func TestWebFindSearchesIndexedPages(t *testing.T) {
	idx := newTestIndex(t)
	_ = idx.Add(pageindex.Page{URL: "https://www.fedlex.admin.ch/or", Title: "OR", Markdown: "## Art. 335c\n\nKündigungsfrist von einem Monat im ersten Dienstjahr."})
	tool := NewWebFindTool(idx)

	out, _ := tool.Execute(args(t, map[string]any{"query": "Kündigungsfrist", "site": "fedlex.admin.ch"}))
	if !strings.Contains(out, "[1] OR › Art. 335c") || !strings.Contains(out, "einem Monat") {
		t.Errorf("unexpected output:\n%s", out)
	}
	out, _ = tool.Execute(args(t, map[string]any{"query": "Kündigungsfrist", "url": "https://other.ch/page"}))
	if !strings.Contains(out, "No passages") || !strings.Contains(out, "read it with webclaw first") {
		t.Errorf("unexpected output: %s", out)
	}
}

// ── site_search ──────────────────────────────────────────────────────────────

type fakeCrawler struct{ calls int }

func (f *fakeCrawler) Crawl(_ context.Context, start string, _ crawl.Options, visit func(crawl.Page)) (int, error) {
	f.calls++
	visit(crawl.Page{URL: "https://www.shop.ch/help/returns", Article: &extract.Article{Title: "Returns", Markdown: "You can return items within 30 days."}})
	visit(crawl.Page{URL: "https://www.shop.ch/help/shipping", Article: &extract.Article{Title: "Shipping", Markdown: "We ship to Switzerland and Liechtenstein."}})
	return 2, nil
}

func TestSiteSearchCrawlsOnceAndSearches(t *testing.T) {
	c := &fakeCrawler{}
	tool := NewSiteSearchTool(c, newTestIndex(t))
	in := args(t, map[string]any{"url": "https://www.shop.ch/help/", "query": "return within days"})

	out, _ := tool.Execute(in)
	if !strings.Contains(out, "Read 2 pages of shop.ch.") || !strings.Contains(out, "[1] Returns") {
		t.Errorf("unexpected output:\n%s", out)
	}
	tool.Execute(in)
	if c.calls != 1 {
		t.Errorf("a recently crawled site should not be crawled again, crawled %d times", c.calls)
	}
}

// ── browser ──────────────────────────────────────────────────────────────────

type fakeSession struct {
	last   string
	closed bool
}

func (f *fakeSession) state(action string) (*browser.State, error) {
	f.last = action
	return &browser.State{URL: "https://x.ch/", Title: "X", Snapshot: "- button \"Go\" [ref=e1]\n", Truncated: action == "scroll"}, nil
}
func (f *fakeSession) Navigate(_ context.Context, u string) (*browser.State, error) {
	return f.state("navigate " + u)
}
func (f *fakeSession) Snapshot(context.Context) (*browser.State, error) { return f.state("snapshot") }
func (f *fakeSession) Click(_ context.Context, ref string) (*browser.State, error) {
	return f.state("click " + ref)
}
func (f *fakeSession) Type(_ context.Context, ref, text string, submit bool) (*browser.State, error) {
	if submit {
		return f.state("type " + ref + " " + text + " submit")
	}
	return f.state("type " + ref + " " + text)
}
func (f *fakeSession) Select(_ context.Context, ref, option string) (*browser.State, error) {
	return f.state("select " + ref + " " + option)
}
func (f *fakeSession) Press(_ context.Context, key string) (*browser.State, error) {
	return f.state("press " + key)
}
func (f *fakeSession) Scroll(_ context.Context, d string) (*browser.State, error) {
	return f.state("scroll")
}
func (f *fakeSession) Back(context.Context) (*browser.State, error) { return f.state("back") }
func (f *fakeSession) HTML(context.Context) (string, string, error) {
	return "https://x.ch/article", "<html><body><article><h1>Title</h1><p>" + strings.Repeat("Readable text. ", 40) + "</p></article></body></html>", nil
}
func (f *fakeSession) Close() error { f.closed = true; return nil }

func TestBrowserToolDispatchesActions(t *testing.T) {
	s := &fakeSession{}
	idx := newTestIndex(t)
	tool := NewBrowserTool(s, extract.NewDefaultChain(), idx)

	cases := map[string]string{
		`{"action":"navigate","url":"https://x.ch"}`:             "navigate https://x.ch",
		`{"action":"click","ref":"e1"}`:                          "click e1",
		`{"action":"type","ref":"e2","text":"hi","submit":true}`: "type e2 hi submit",
		`{"action":"select","ref":"e3","text":"Zürich"}`:         "select e3 Zürich",
		`{"action":"press","key":"Enter"}`:                       "press Enter",
		`{"action":"back"}`:                                      "back",
	}
	for in, want := range cases {
		out, _ := tool.Execute(in)
		if s.last != want || !strings.Contains(out, `button "Go" [ref=e1]`) || !strings.HasPrefix(out, "URL: https://x.ch/\nTitle: X") {
			t.Errorf("%s: session got %q, output %q", in, s.last, out)
		}
	}
	if out, _ := tool.Execute(`{"action":"scroll"}`); !strings.Contains(out, "Snapshot truncated") {
		t.Errorf("missing truncation note: %q", out)
	}

	out, _ := tool.Execute(`{"action":"read"}`)
	if !strings.Contains(out, "Readable text.") {
		t.Errorf("read output: %q", out)
	}
	if hits, _ := idx.Search(pageindex.Query{Text: "readable"}); len(hits) == 0 {
		t.Error("read pages should be indexed for web_find")
	}

	if out, _ := tool.Execute(`{"action":"navigate"}`); !strings.Contains(out, "needs a url") {
		t.Errorf("got %q", out)
	}
	if out, _ := tool.Execute(`{"action":"fly"}`); !strings.Contains(out, "unknown action") {
		t.Errorf("got %q", out)
	}
	tool.Execute(`{"action":"close"}`)
	if !s.closed {
		t.Error("close should close the session")
	}
}
