package tools

import "strings"

// botChallengeSignals are substrings that indicate the server blocked the request.
var botChallengeSignals = []string{
	"captcha",
	"verify you are human",
	"access denied",
	"please enable javascript",
	"enable javascript and cookies",
	"just a moment",         // Cloudflare
	"checking your browser", // Cloudflare
	"ddos protection",
	"rate limit",
	"too many requests",
	"please verify",
}

// jsRequiredSignals indicate the page needs JavaScript to render content
var jsRequiredSignals = []string{
	"javascript is required",
	"javascript muss aktiviert",   // German
	"javascript doit être activé", // French
	"javascript deve essere",      // Italian
	"enable javascript",
	"requires javascript",
	"you need to enable javascript",
	"this site requires javascript",
	"noscript",
	"<app-root></app-root>",   // Angular empty root
	"<div id=\"root\"></div>", // React empty root
	"<div id=\"app\"></div>",  // Vue empty root
}

func looksLikeBlockedPage(body string) bool {
	lower := strings.ToLower(body)
	for _, s := range botChallengeSignals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func looksLikeJSRequired(body string) bool {
	lower := strings.ToLower(body)
	for _, s := range jsRequiredSignals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// needsJSRendering checks if a URL is known to require JS rendering
func needsJSRendering(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, hint := range jsRenderingHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func looksLikeHTML(s string) bool {
	p := strings.TrimSpace(s)
	if len(p) > 512 {
		p = p[:512]
	}
	p = strings.ToLower(p)
	return strings.HasPrefix(p, "<!doctype html") ||
		strings.HasPrefix(p, "<html") ||
		strings.Contains(p, "<body")
}

// jsRenderingHints are domain patterns that are known to require JS rendering
var jsRenderingHints = []string{
	"fedlex.admin.ch",
}
