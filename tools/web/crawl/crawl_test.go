package crawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"pedro/tools/web/extract"
)

type titleExtractor struct{}

func (titleExtractor) Name() string { return "test" }
func (titleExtractor) Extract(html, pageURL string) (*extract.Article, error) {
	return &extract.Article{Markdown: html}, nil
}

func site() *httptest.Server {
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body>"+body+"</body></html>")
		}
	}
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /docs/private/\n")
	})
	mux.HandleFunc("/docs/", page(`<a href="/docs/a">a</a> <a href="b">b</a> <a href="/docs/private/x">private</a>
		<a href="/blog/">outside</a> <a href="/docs/file.pdf">pdf</a> <a href="https://other.example/">external</a> <a href="/docs/a#frag">again</a>`))
	mux.HandleFunc("/docs/a", page(`<a href="/docs/c">c</a>`))
	mux.HandleFunc("/docs/b", page(`b`))
	mux.HandleFunc("/docs/c", page(`c`))
	mux.HandleFunc("/docs/private/x", page(`secret`))
	mux.HandleFunc("/blog/", page(`blog`))
	return httptest.NewServer(mux)
}

func crawlPaths(t *testing.T, srv *httptest.Server, opts Options) []string {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	c := NewColly(http.DefaultTransport, "test", titleExtractor{})
	n, err := c.Crawl(context.Background(), srv.URL+"/docs/", opts, func(p Page) {
		u, _ := url.Parse(p.URL)
		mu.Lock()
		paths = append(paths, u.Path)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(paths) {
		t.Errorf("returned %d pages but visited %d", n, len(paths))
	}
	sort.Strings(paths)
	return paths
}

func TestCrawlStaysInScopeAndRespectsRobots(t *testing.T) {
	srv := site()
	defer srv.Close()
	got := crawlPaths(t, srv, Options{MaxPages: 20, MaxDepth: 3, Timeout: 10 * time.Second})
	want := []string{"/docs/", "/docs/a", "/docs/b", "/docs/c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("visited %v, want %v", got, want)
	}
}

func TestCrawlStopsAtMaxPages(t *testing.T) {
	srv := site()
	defer srv.Close()
	if got := crawlPaths(t, srv, Options{MaxPages: 2, MaxDepth: 3, Timeout: 10 * time.Second}); len(got) != 2 {
		t.Errorf("visited %v, want 2 pages", got)
	}
}

func TestScopeOf(t *testing.T) {
	tests := map[string]string{
		"https://x.ch":            "/",
		"https://x.ch/":           "/",
		"https://x.ch/intro":      "/",
		"https://x.ch/docs/intro": "/docs/",
		"https://x.ch/docs/":      "/docs/",
	}
	for raw, want := range tests {
		u, _ := url.Parse(raw)
		if got := scopeOf(u).prefix; got != want {
			t.Errorf("scopeOf(%s).prefix = %q, want %q", raw, got, want)
		}
	}
}
