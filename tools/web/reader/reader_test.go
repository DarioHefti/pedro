package reader

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"pedro/documentparser"
	"pedro/tools/web/browser"
	"pedro/tools/web/extract"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func serve(status int, contentType, body string, header ...string) doerFunc {
	return func(r *http.Request) (*http.Response, error) {
		h := http.Header{"Content-Type": {contentType}}
		for i := 0; i+1 < len(header); i += 2 {
			h.Set(header[i], header[i+1])
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
}

type fakeRenderer struct {
	title string
	html  string
	calls atomic.Int32
}

func (f *fakeRenderer) Render(_ context.Context, rawURL string, _ browser.RenderOptions) (*browser.Rendered, error) {
	f.calls.Add(1)
	return &browser.Rendered{URL: rawURL, Title: f.title, HTML: f.html}, nil
}

// textExtractor returns the body text between <p> tags so tests control length.
type textExtractor struct{}

func (textExtractor) Name() string { return "test" }
func (textExtractor) Extract(html, _ string) (*extract.Article, error) {
	start, end := strings.Index(html, "<p>"), strings.Index(html, "</p>")
	if start < 0 || end < start {
		return nil, extract.ErrNoContent
	}
	return &extract.Article{Markdown: html[start+3 : end]}, nil
}

var (
	longPage  = "<html><body><p>" + strings.Repeat("content ", 100) + "</p></body></html>"
	shortPage = "<html><body><p>Loading…</p></body></html>"
)

func TestStaticPageIsNotRendered(t *testing.T) {
	r := &fakeRenderer{html: longPage}
	page, err := New(serve(200, "text/html", longPage), nil, r, textExtractor{}, nil).Read(context.Background(), "https://example.com/a")
	if err != nil || page.Rendered || r.calls.Load() != 0 {
		t.Fatalf("page=%+v err=%v renders=%d", page, err, r.calls.Load())
	}
}

func TestShortPageIsRendered(t *testing.T) {
	r := &fakeRenderer{html: longPage}
	page, err := New(serve(200, "text/html", shortPage), nil, r, textExtractor{}, nil).Read(context.Background(), "https://example.com/a")
	if err != nil || !page.Rendered {
		t.Fatalf("expected rendered page, got %+v err=%v", page, err)
	}
}

func TestShortPageKeptWhenRenderingIsNotBetter(t *testing.T) {
	r := &fakeRenderer{html: "<html><p>x</p></html>"}
	page, err := New(serve(200, "text/html", shortPage), nil, r, textExtractor{}, nil).Read(context.Background(), "https://example.com/a")
	if err != nil || page.Rendered || page.Article.Markdown != "Loading…" {
		t.Fatalf("expected the static page, got %+v err=%v", page, err)
	}
}

func TestBotCheckIsRendered(t *testing.T) {
	challenge := "<html><head><title>Just a moment...</title></head><body><p>" + strings.Repeat("x ", 300) + "</p></body></html>"
	for name, d := range map[string]doerFunc{
		"status":         serve(403, "text/html", longPage),
		"cf-mitigated":   serve(200, "text/html", longPage, "cf-mitigated", "challenge"),
		"challenge page": serve(200, "text/html", challenge),
	} {
		r := &fakeRenderer{html: longPage}
		page, err := New(d, nil, r, textExtractor{}, nil).Read(context.Background(), "https://example.com/a")
		if err != nil || !page.Rendered {
			t.Errorf("%s: expected rendered page, got %+v err=%v", name, page, err)
		}
	}
}

func TestUnpassableBotCheckIsReported(t *testing.T) {
	r := &fakeRenderer{title: "Just a moment...", html: longPage}
	_, err := New(serve(403, "text/html", ""), nil, r, textExtractor{}, nil).Read(context.Background(), "https://example.com/a")
	if !errors.Is(err, ErrBotCheck) {
		t.Fatalf("expected bot check error, got %v", err)
	}
}

func TestCloudflareRetryWithFallbackClient(t *testing.T) {
	var fallbackUA string
	fallback := doerFunc(func(r *http.Request) (*http.Response, error) {
		fallbackUA = r.Header.Get("User-Agent")
		return serve(200, "text/html", longPage)(r)
	})
	page, err := New(serve(403, "text/html", "", "cf-mitigated", "challenge"), fallback, nil, textExtractor{}, nil).
		Read(context.Background(), "https://example.com/a")
	if err != nil || page.Rendered || fallbackUA != "pedro" {
		t.Fatalf("page=%+v err=%v ua=%q", page, err, fallbackUA)
	}
}

func TestJSOnlyHostGoesStraightToBrowser(t *testing.T) {
	called := false
	d := doerFunc(func(r *http.Request) (*http.Response, error) { called = true; return nil, errors.New("unused") })
	r := &fakeRenderer{html: longPage}
	page, err := New(d, nil, r, textExtractor{}, nil).Read(context.Background(), "https://www.fedlex.admin.ch/eli/cc/27/317_321_377/de")
	if err != nil || !page.Rendered || called {
		t.Fatalf("page=%+v err=%v httpCalled=%v", page, err, called)
	}
}

func TestNonHTMLContent(t *testing.T) {
	page, err := New(serve(200, "application/json", `{"a":1}`), nil, nil, textExtractor{}, nil).Read(context.Background(), "https://api.example.com/x")
	if err != nil || page.Article.Markdown != `{"a":1}` {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	_, err = New(serve(200, "image/png", "\x89PNG"), nil, nil, textExtractor{}, nil).Read(context.Background(), "https://example.com/x.png")
	if !errors.Is(err, ErrUnsupportedContent) {
		t.Fatalf("expected ErrUnsupportedContent, got %v", err)
	}
}

func TestInvalidURL(t *testing.T) {
	for _, u := range []string{"", "example.com", "file:///etc/passwd", "javascript:alert(1)"} {
		if _, err := New(nil, nil, nil, textExtractor{}, nil).Read(context.Background(), u); err == nil {
			t.Errorf("expected error for %q", u)
		}
	}
}

type fakeDocuments struct{ path string }

func (f *fakeDocuments) Parse(path string) (*documentparser.Result, error) {
	f.path = path
	return &documentparser.Result{Format: "pdf", Lines: []string{"--- Page 1 ---", "Annual report", "--- Page 2 ---", "Revenue grew."}}, nil
}

func TestReadsDocumentsWithTheDocumentParser(t *testing.T) {
	for _, tc := range []struct{ contentType, url string }{
		{"application/pdf", "https://example.com/report"},
		{"application/octet-stream", "https://example.com/files/report.pdf"},
	} {
		docs := &fakeDocuments{}
		page, err := New(serve(200, tc.contentType, "%PDF-1.7 ..."), nil, nil, textExtractor{}, docs).Read(context.Background(), tc.url)
		if err != nil {
			t.Fatalf("%s: %v", tc.contentType, err)
		}
		if !strings.HasSuffix(docs.path, ".pdf") {
			t.Errorf("parser got %q, want a .pdf temp file", docs.path)
		}
		if _, err := os.Stat(docs.path); !os.IsNotExist(err) {
			t.Errorf("temp file %s was not removed", docs.path)
		}
		want := "## Page 1\n\nAnnual report\n\n## Page 2\n\nRevenue grew."
		if page.Article.Markdown != want || page.Article.Extractor != "pdf" {
			t.Errorf("got %q (%s)", page.Article.Markdown, page.Article.Extractor)
		}
	}
}
