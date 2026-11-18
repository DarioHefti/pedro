//go:build live

package fetch

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Run with: go test -tags live ./tools/web/fetch/
func TestLiveFingerprintLooksLikeChrome(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := Get(ctx, NewBrowserClient(Options{}), "https://tls.peet.ws/api/all", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		HTTPVersion string `json:"http_version"`
		TLS         struct {
			JA4 string `json:"ja4"`
		} `json:"tls"`
		HTTP2 struct {
			Akamai string `json:"akamai_fingerprint"`
		} `json:"http2"`
	}
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		t.Fatal(err)
	}
	t.Logf("http=%s ja4=%s akamai=%s", data.HTTPVersion, data.TLS.JA4, data.HTTP2.Akamai)
	if data.HTTPVersion != "h2" {
		t.Errorf("expected HTTP/2, got %s", data.HTTPVersion)
	}
	if data.HTTP2.Akamai != "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p" {
		t.Errorf("HTTP/2 fingerprint differs from Chrome: %s", data.HTTP2.Akamai)
	}
	if data.TLS.JA4[:10] != "t13d1516h2" {
		t.Errorf("TLS fingerprint differs from Chrome: %s", data.TLS.JA4)
	}
}
