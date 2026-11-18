// Package weburl holds URL helpers shared by the web components.
package weburl

import (
	"net/url"
	"strings"
)

var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "msclkid": true, "ref": true, "ref_src": true, "srsltid": true,
}

// Key returns a comparison key for a URL: scheme, "www.", fragments,
// trailing slashes and tracking parameters are ignored. Returns "" for
// non-HTTP URLs.
func Key(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if trackingParams[lk] || strings.HasPrefix(lk, "utm_") {
			q.Del(k)
		}
	}
	key := Host(raw) + strings.TrimRight(u.EscapedPath(), "/")
	if enc := q.Encode(); enc != "" {
		key += "?" + enc
	}
	return key
}

// Host returns the lower-cased host of raw without a leading "www.".
func Host(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
