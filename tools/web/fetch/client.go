// Package fetch provides an HTTP client that is indistinguishable from a real
// Chrome browser at the network level (TLS ClientHello, HTTP/2 SETTINGS and
// priority frames, header order and default navigation headers).
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/imroc/req/v3/http2"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/publicsuffix"
)

// Doer is the minimal HTTP abstraction every web component depends on.
// *http.Client satisfies it, which keeps all components easy to test.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Options configures NewBrowserClient.
type Options struct {
	Profile      Profile
	Timeout      time.Duration
	MaxRedirects int
}

// Chrome's HTTP/2 fingerprint (Akamai format "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p").
var (
	chromeHTTP2Settings = []http2.Setting{
		{ID: http2.SettingHeaderTableSize, Val: 65536},
		{ID: http2.SettingEnablePush, Val: 0},
		{ID: http2.SettingInitialWindowSize, Val: 6291456},
		{ID: http2.SettingMaxHeaderListSize, Val: 262144},
	}
	chromeHeaderPriority = http2.PriorityParam{StreamDep: 0, Exclusive: true, Weight: 255}
	chromeHeaderOrder    = []string{
		"host",
		"content-length",
		"cache-control",
		"sec-ch-ua",
		"sec-ch-ua-mobile",
		"sec-ch-ua-platform",
		"origin",
		"content-type",
		"upgrade-insecure-requests",
		"user-agent",
		"accept",
		"sec-fetch-site",
		"sec-fetch-mode",
		"sec-fetch-user",
		"sec-fetch-dest",
		"referer",
		"accept-encoding",
		"accept-language",
		"cookie",
		"priority",
	}
)

// NewBrowserClient returns a standard *http.Client that impersonates Chrome.
// It keeps cookies per site, like a browser session would.
func NewBrowserClient(opts Options) *http.Client {
	if opts.Profile.UserAgent == "" {
		opts.Profile = ChromeProfile()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxRedirects == 0 {
		opts.MaxRedirects = 10
	}

	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	maxRedirects := opts.MaxRedirects
	return &http.Client{
		Transport: NewBrowserTransport(opts.Profile),
		Jar:       jar,
		Timeout:   opts.Timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// NewBrowserTransport returns a RoundTripper with Chrome's TLS and HTTP/2
// fingerprint that also fills in Chrome's default navigation headers.
func NewBrowserTransport(profile Profile) http.RoundTripper {
	if profile.UserAgent == "" {
		profile = ChromeProfile()
	}
	transport := req.C().
		SetTLSFingerprint(utls.HelloChrome_133).
		SetHTTP2SettingsFrame(chromeHTTP2Settings...).
		SetHTTP2ConnectionFlow(15663105).
		SetHTTP2HeaderPriority(chromeHeaderPriority).
		SetCommonPseudoHeaderOder(":method", ":authority", ":scheme", ":path").
		SetCommonHeaderOrder(chromeHeaderOrder...).
		EnableAutoDecompress().
		GetTransport()
	return &browserHeaders{next: transport, profile: profile}
}

// browserHeaders fills in the headers Chrome sends on a top-level navigation,
// leaving any header the caller set explicitly untouched.
type browserHeaders struct {
	next    http.RoundTripper
	profile Profile
}

func (b *browserHeaders) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	h := r.Header
	setDefault := func(key, value string) {
		if h.Get(key) == "" {
			h.Set(key, value)
		}
	}
	setDefault("User-Agent", b.profile.UserAgent)
	setDefault("Sec-Ch-Ua", b.profile.SecCHUA)
	setDefault("Sec-Ch-Ua-Mobile", "?0")
	setDefault("Sec-Ch-Ua-Platform", b.profile.Platform)
	setDefault("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	setDefault("Accept-Language", b.profile.AcceptLanguage)
	setDefault("Accept-Encoding", "gzip, deflate, br, zstd")
	setDefault("Sec-Fetch-Site", "none")
	setDefault("Sec-Fetch-Mode", "navigate")
	setDefault("Sec-Fetch-Dest", "document")
	if h.Get("Sec-Fetch-Mode") == "navigate" {
		setDefault("Sec-Fetch-User", "?1")
		setDefault("Upgrade-Insecure-Requests", "1")
		setDefault("Priority", "u=0, i")
	}
	return b.next.RoundTrip(r)
}

// Response is a fully read HTTP response.
type Response struct {
	URL         string // final URL after redirects
	StatusCode  int
	Header      http.Header
	ContentType string
	Body        []byte
}

// Get downloads rawURL, reading at most maxBytes of the body.
func Get(ctx context.Context, d Doer, rawURL string, maxBytes int64) (*Response, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return Do(d, r, maxBytes)
}

// Do executes r and reads at most maxBytes of the body.
func Do(d Doer, r *http.Request, maxBytes int64) (*Response, error) {
	resp, err := d.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	finalURL := r.URL.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return &Response{
		URL:         finalURL,
		StatusCode:  resp.StatusCode,
		Header:      resp.Header,
		ContentType: strings.ToLower(resp.Header.Get("Content-Type")),
		Body:        body,
	}, nil
}
