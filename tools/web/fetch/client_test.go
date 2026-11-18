package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserHeadersFillDefaultsWithoutOverriding(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer srv.Close()

	profile := ChromeProfile()
	client := &http.Client{Transport: &browserHeaders{next: http.DefaultTransport, profile: profile}}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := Do(client, req, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}

	if got.Get("Accept") != "application/json" {
		t.Errorf("explicit header was overridden: %q", got.Get("Accept"))
	}
	if got.Get("User-Agent") != profile.UserAgent || got.Get("Sec-Ch-Ua") != profile.SecCHUA {
		t.Errorf("missing Chrome identity: %v", got)
	}
	if got.Get("Sec-Fetch-Mode") != "navigate" || got.Get("Upgrade-Insecure-Requests") != "1" {
		t.Errorf("missing navigation headers: %v", got)
	}
}

func TestChromeProfileIsConsistent(t *testing.T) {
	p := ChromeProfile()
	if !strings.Contains(p.UserAgent, "Chrome/145.0.0.0") || !strings.Contains(p.SecCHUA, `"Google Chrome";v="145"`) {
		t.Errorf("user agent and client hints disagree: %+v", p)
	}
	for goos, platform := range map[string]string{"darwin": `"macOS"`, "windows": `"Windows"`, "linux": `"Linux"`} {
		if _, got := platformTokens(goos); got != platform {
			t.Errorf("platform for %s = %s", goos, got)
		}
	}
}

func TestDoLimitsBodyAndReportsFinalURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusFound)
			return
		}
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()
	resp, err := Get(context.Background(), srv.Client(), srv.URL+"/old", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 10 || !strings.HasSuffix(resp.URL, "/new") {
		t.Errorf("body=%d url=%s", len(resp.Body), resp.URL)
	}
}
