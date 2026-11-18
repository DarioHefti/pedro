package search

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pedro/tools/web/browser"

	"github.com/PuerkitoBio/goquery"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) doerFunc {
	return func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
}

type fakeRenderer struct {
	html     string
	selector string
	calls    atomic.Int32
}

func (f *fakeRenderer) Render(_ context.Context, rawURL string, opts browser.RenderOptions) (*browser.Rendered, error) {
	f.calls.Add(1)
	f.selector = opts.ReadySelector
	return &browser.Rendered{URL: rawURL, HTML: f.html}, nil
}

type fakeProvider struct {
	name    string
	results []Result
	err     error
	calls   atomic.Int32
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Search(context.Context, string) ([]Result, error) {
	f.calls.Add(1)
	return f.results, f.err
}

func doc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := parseDocument(html)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ── fixtures (structure copied from the live result pages) ───────────────────

const ddgHTML = `<div id="links" class="results">
<div class="result results_links results_links_deep result--ad"><h2 class="result__title"><a class="result__a" href="https://duckduckgo.com/y.js?ad_domain=x">Ad</a></h2></div>
<div class="result results_links results_links_deep web-result"><div class="links_main links_deep result__body">
  <h2 class="result__title"><a rel="nofollow" class="result__a" href="https://pkg.go.dev/net/http">http package - net/http</a></h2>
  <a class="result__snippet" href="https://pkg.go.dev/net/http">Package <b>http</b> provides HTTP client and server implementations.</a>
</div></div>
<div class="result results_links web-result"><h2 class="result__title"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgobyexample.com%2Fhttp-client&amp;rut=abc">Go by Example</a></h2></div>
</div>`

const braveHTML = `<main><section id="mixed-main">
<div class="snippet svelte-jmfu5f" data-pos="0" data-type="web"><div class="result-body"><div class="result-wrapper"><div class="result-content">
  <a href="https://pkg.go.dev/net/http" class="l1"><div class="site-name-wrapper"><div>Go Packages</div></div>
  <div class="title search-snippet-title line-clamp-1" title="http package - net/http - Go Packages">http package - net/http - Go Packages</div></a>
  <div class="generic-snippet"><div class="content desktop-default-regular">Package http provides HTTP client and server implementations.</div></div>
</div></div></div></div>
<div class="snippet" data-type="video"><div class="result-content"><a href="https://youtube.com/x"><div class="search-snippet-title">Video</div></a></div></div>
</section></main>`

const startpageHTML = `<div class="w-gl"><div class="result css-1lr2kir-WGl"><div class="upper"><a href="https://pkg.go.dev/net/http" class="favicon-link"></a></div>
<a class="result-title result-link css-11za2kn" href="https://pkg.go.dev/net/http"><style>.css-1bpj8yc{line-height:18px;}</style><h2 class="wgl-title">net/http - Go Packages</h2></a>
<p class="description">A Client is an <b>HTTP client</b>.</p></div>
<div class="result h css-1lr2kir-WGl"><a class="result-title" href="/clev?event=x">Tracking</a></div></div>`

const mojeekHTML = `<ul class="results-standard"><li><a class="ob" href="https://pkg.go.dev/net/http">pkg.go.dev</a>
<h2><a class="title" href="https://pkg.go.dev/net/http">http package</a></h2><p class="s">Package http provides...</p></li></ul>`

func TestParsers(t *testing.T) {
	tests := []struct {
		name  string
		parse func(*goquery.Document) []Result
		html  string
		want  []Result
	}{
		{"duckduckgo", parseDuckDuckGo, ddgHTML, []Result{
			{Title: "http package - net/http", URL: "https://pkg.go.dev/net/http", Snippet: "Package http provides HTTP client and server implementations."},
			{Title: "Go by Example", URL: "https://gobyexample.com/http-client"},
		}},
		{"brave", parseBrave, braveHTML, []Result{
			{Title: "http package - net/http - Go Packages", URL: "https://pkg.go.dev/net/http", Snippet: "Package http provides HTTP client and server implementations."},
		}},
		{"startpage", parseStartpage, startpageHTML, []Result{
			{Title: "net/http - Go Packages", URL: "https://pkg.go.dev/net/http", Snippet: "A Client is an HTTP client."},
		}},
		{"mojeek", parseMojeek, mojeekHTML, []Result{
			{Title: "http package", URL: "https://pkg.go.dev/net/http", Snippet: "Package http provides..."},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.parse(doc(t, tt.html))
			if len(got) != len(tt.want) {
				t.Fatalf("got %d results %+v, want %d", len(got), got, len(tt.want))
			}
			for i := range got {
				if got[i].Title != tt.want[i].Title || got[i].URL != tt.want[i].URL || got[i].Snippet != tt.want[i].Snippet {
					t.Errorf("result %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractDDGURL(t *testing.T) {
	tests := map[string]string{
		"//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=something":                        "https://example.com",
		"https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath%3Fq%3D123&rut=something": "https://example.com/path?q=123",
		"https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fa%2520b":                      "https://example.com/a%20b",
		"https://example.com":             "https://example.com",
		"//example.com":                   "//example.com",
		"":                                "",
		"https://google.com/l/?uddg=test": "https://google.com/l/?uddg=test",
	}
	for in, want := range tests {
		if got := extractDDGURL(in); got != want {
			t.Errorf("extractDDGURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHTMLEngineFallsBackToBrowserAndRemembers(t *testing.T) {
	var httpCalls atomic.Int32
	blocked := func(r *http.Request) (*http.Response, error) {
		httpCalls.Add(1)
		return respond(http.StatusTooManyRequests, "captcha")(r)
	}
	renderer := &fakeRenderer{html: ddgHTML}
	e := newHTMLEngine("duckduckgo", func(q string) string { return "https://example.com/?q=" + q }, parseDuckDuckGo, "div.result", doerFunc(blocked), renderer)

	for i := 0; i < 2; i++ {
		results, err := e.Search(context.Background(), "go")
		if err != nil || len(results) != 2 {
			t.Fatalf("search %d: results=%v err=%v", i, results, err)
		}
	}
	if httpCalls.Load() != 1 {
		t.Errorf("expected the blocked HTTP path to be tried once, got %d", httpCalls.Load())
	}
	if renderer.calls.Load() != 2 {
		t.Errorf("expected 2 browser renders, got %d", renderer.calls.Load())
	}
	if renderer.selector != "div.result" {
		t.Errorf("browser should wait for the results selector, got %q", renderer.selector)
	}
}

func TestHTMLEngineWithoutRendererReportsBlocked(t *testing.T) {
	for _, page := range []string{
		"<html><head><title>Captcha</title></head><body>Please verify</body></html>",
		`<html><body><div class="anomaly-modal__mask"></div></body></html>`,
		`<html><body><iframe src="https://challenges.cloudflare.com/x"></iframe></body></html>`,
	} {
		e := newHTMLEngine("x", func(q string) string { return "https://example.com/" }, parseDuckDuckGo, "div.result", respond(http.StatusOK, page), nil)
		if _, err := e.Search(context.Background(), "go"); !errors.Is(err, ErrBlocked) {
			t.Errorf("expected ErrBlocked for %q, got %v", page, err)
		}
	}
}

func TestHTMLEngineTreatsEmptyResultsAsNoHits(t *testing.T) {
	renderer := &fakeRenderer{html: ddgHTML}
	empty := `<html><head><title>xyz at DuckDuckGo</title></head><body><div class="no-results">No results.</div></body></html>`
	e := newHTMLEngine("duckduckgo", func(q string) string { return "https://example.com/" }, parseDuckDuckGo, "div.result", respond(http.StatusOK, empty), renderer)
	results, err := e.Search(context.Background(), "a query nobody ever searched")
	if err != nil || len(results) != 0 {
		t.Fatalf("expected no hits and no error, got %v %v", results, err)
	}
	if renderer.calls.Load() != 0 {
		t.Error("an empty results page must not escalate to the browser")
	}
}

func TestFuseRanksConsensusFirst(t *testing.T) {
	a := &fakeProvider{name: "a", results: []Result{
		{Title: "X", URL: "https://x.com/"},
		{Title: "Y", URL: "https://www.y.com/page?utm_source=a", Snippet: "short"},
	}}
	b := &fakeProvider{name: "b", results: []Result{
		{Title: "Y", URL: "http://y.com/page", Snippet: "a much longer snippet"},
		{Title: "Z", URL: "https://z.com/"},
		{Title: "Y again", URL: "https://y.com/page#top"},
	}}
	m := NewMetaSearcher(time.Second, time.Minute, Weighted{a, 1}, Weighted{b, 1})
	got, statuses := m.Search(context.Background(), "q", 10)

	if len(got) != 3 {
		t.Fatalf("expected 3 merged results, got %+v", got)
	}
	if got[0].Title != "Y" || strings.Join(got[0].Engines, ",") != "a,b" {
		t.Errorf("expected Y (found by a and b) first, got %+v", got[0])
	}
	if got[0].Snippet != "a much longer snippet" {
		t.Errorf("expected the longest snippet, got %q", got[0].Snippet)
	}
	if statuses[0].Results != 2 || statuses[1].Results != 3 {
		t.Errorf("unexpected statuses %+v", statuses)
	}
}

func TestMetaSearcherCoolsDownFailingProvider(t *testing.T) {
	bad := &fakeProvider{name: "bad", err: ErrBlocked}
	good := &fakeProvider{name: "good", results: []Result{{Title: "A", URL: "https://a.com"}}}
	m := NewMetaSearcher(time.Second, time.Hour, Weighted{bad, 1}, Weighted{good, 1})

	m.Search(context.Background(), "q", 5)
	_, statuses := m.Search(context.Background(), "q", 5)
	if bad.calls.Load() != 1 {
		t.Errorf("failing provider should be skipped while cooling down, called %d times", bad.calls.Load())
	}
	if !statuses[0].Skipped {
		t.Errorf("expected skipped status, got %+v", statuses[0])
	}

	flaky := &fakeProvider{name: "flaky", err: errors.New("connection reset")}
	m = NewMetaSearcher(time.Second, time.Hour, Weighted{flaky, 1}, Weighted{good, 1})
	m.Search(context.Background(), "q", 5)
	m.Search(context.Background(), "q", 5)
	if flaky.calls.Load() != 2 {
		t.Errorf("only bot checks should pause a provider, flaky was called %d times", flaky.calls.Load())
	}
}

func TestMetaSearcherStopsWaitingAfterQuorum(t *testing.T) {
	a := &fakeProvider{name: "a", results: []Result{{Title: "A", URL: "https://a.com"}}}
	b := &fakeProvider{name: "b", results: []Result{{Title: "B", URL: "https://b.com"}}}
	m := NewMetaSearcher(time.Minute, time.Minute, Weighted{a, 1}, Weighted{b, 1}, Weighted{slowProvider{}, 1})
	b.results = nil // an answer without hits still counts towards the quorum
	m.grace = 20 * time.Millisecond

	start := time.Now()
	got, statuses := m.Search(context.Background(), "q", 5)
	if time.Since(start) > time.Second {
		t.Fatalf("waited %s for a straggler", time.Since(start))
	}
	if len(got) != 1 || !errors.Is(statuses[2].Err, ErrTooSlow) {
		t.Fatalf("got %+v statuses %+v", got, statuses)
	}
}

func TestMetaSearcherReturnsPartialResultsOnTimeout(t *testing.T) {
	slow := &slowProvider{}
	fast := &fakeProvider{name: "fast", results: []Result{{Title: "A", URL: "https://a.com"}}}
	m := NewMetaSearcher(50*time.Millisecond, time.Minute, Weighted{slow, 1}, Weighted{fast, 1})
	got, statuses := m.Search(context.Background(), "q", 5)
	if len(got) != 1 || statuses[0].Err == nil {
		t.Fatalf("expected the fast result and a timeout status, got %+v %+v", got, statuses)
	}
}

type slowProvider struct{}

func (slowProvider) Name() string { return "slow" }
func (slowProvider) Search(ctx context.Context, _ string) ([]Result, error) {
	<-ctx.Done()
	time.Sleep(10 * time.Millisecond)
	return nil, ctx.Err()
}

func TestWikipedia(t *testing.T) {
	body := `{"query":{"search":[{"title":"Go (programming language)","snippet":"<span class=\"searchmatch\">Go</span> is a language"}]}}`
	results, err := NewWikipedia("en", respond(http.StatusOK, body)).Search(context.Background(), "go")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%v err=%v", results, err)
	}
	r := results[0]
	if r.URL != "https://en.wikipedia.org/wiki/Go_%28programming_language%29" || r.Snippet != "Go is a language" {
		t.Errorf("unexpected result %+v", r)
	}
}

func TestOpenAlex(t *testing.T) {
	body := `{"results":[{"id":"https://openalex.org/W1","doi":"https://doi.org/10.1234/test","display_name":"Test Paper",
		"publication_year":2024,"cited_by_count":42,"open_access":{"is_oa":true,"oa_url":"https://example.com/p.pdf"},
		"primary_location":{"source":{"display_name":"Test Journal"}}},{"id":"https://openalex.org/W2","display_name":""}]}`
	results, err := NewOpenAlex(respond(http.StatusOK, body)).Search(context.Background(), "test")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%v err=%v", results, err)
	}
	want := Result{
		Title:   "[Scholarly] Test Paper",
		URL:     "https://doi.org/10.1234/test",
		Snippet: "Published 2024 · Cited by 42 · Test Journal · Open Access (https://example.com/p.pdf)",
	}
	if results[0].Title != want.Title || results[0].URL != want.URL || results[0].Snippet != want.Snippet {
		t.Errorf("got %+v, want %+v", results[0], want)
	}
}
