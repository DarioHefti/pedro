package webclaw

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var noiseTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true,
	"svg": true, "nav": true, "aside": true, "footer": true,
	"header": true, "video": true, "audio": true, "canvas": true,
}

var noiseRoles = map[string]bool{
	"navigation": true, "banner": true, "complementary": true, "contentinfo": true,
}

var noiseClasses = map[string]bool{
	"header": true, "top": true, "navbar": true, "footer": true, "bottom": true,
	"sidebar": true, "modal": true, "popup": true, "overlay": true,
	"ad": true, "ads": true, "advert": true, "lang-selector": true, "language": true,
	"social": true, "social-media": true, "social-links": true, "menu": true,
	"navigation": true, "breadcrumbs": true, "breadcrumb": true, "share": true,
	"widget": true, "cookie": true, "newsletter": true, "subscribe": true,
	"skip-link": true, "sr-only": true, "visually-hidden": true,
	"notification": true, "alert": true, "toast": true,
	"pagination": true, "pager": true, "signup": true,
	"login-form": true, "search-form": true, "related-posts": true, "recommended": true,
}

var noiseIDs = map[string]bool{
	"header": true, "footer": true, "nav": true, "sidebar": true,
	"menu": true, "modal": true, "popup": true, "cookie": true,
	"breadcrumbs": true, "widget": true, "ad": true, "social": true,
	"share": true, "newsletter": true, "subscribe": true,
	"comments": true, "related": true, "recommended": true,
}

var noiseClassPatterns = []string{
	"sidebar", "side", "nav", "navbar", "navigation", "menu", "footer", "header",
	"top", "bottom", "advertisement", "advert", "social", "social-media",
	"social-links", "share", "comment", "cookie", "popup", "modal", "overlay",
	"banner", "breadcrumb", "breadcrumbs", "widget", "lang-selector", "language",
	"newsletter", "subscribe", "related-posts", "recommended", "pagination",
	"pager", "signup", "login-form", "search-form", "notification", "alert",
	"toast", "skip-link", "sr-only", "visually-hidden",
}

var cookieConsentPrefixes = []string{
	"onetrust", "optanon", "ot-sdk", "cookiebot", "cybotcookiebot",
	"cc-", "cookie-law", "gdpr", "consent-", "cmp-",
	"sp_message", "qc-cmp", "trustarc", "evidon",
}

var structuralIDSuffixes = []string{
	"portal", "root", "container", "wrapper", "mount", "app",
}

var noisePrefixClasses = map[string]bool{
	"banner": true, "overlay": true,
}

// isNoise checks if an element is noise by tag, role, class, or id.
func isNoise(el *goquery.Selection) bool {
	tag := goquery.NodeName(el)

	if tag == "body" || tag == "html" {
		return false
	}

	if noiseTags[tag] {
		return true
	}

	// Form heuristic: small forms are noise, page-wrapping forms are content
	if tag == "form" {
		textLen := len(strings.TrimSpace(el.Text()))
		if textLen < 500 {
			return true
		}
		if cls, exists := el.Attr("class"); exists {
			cl := strings.ToLower(cls)
			for _, kw := range []string{"login", "search", "subscribe", "signup", "newsletter", "contact"} {
				if strings.Contains(cl, kw) {
					return true
				}
			}
		}
		return false
	}

	// ARIA role
	if role, exists := el.Attr("role"); exists && noiseRoles[role] {
		return true
	}

	// Class-based detection (exact token matching)
	if cls, exists := el.Attr("class"); exists {
		if isNoiseClass(cls) {
			// Safety valve: large noise elements are likely broken wrappers
			textLen := len(strings.TrimSpace(el.Text()))
			if textLen > 5000 {
				return false
			}
			return true
		}
	}

	// ID-based detection
	if id, exists := el.Attr("id"); exists {
		idLower := strings.ToLower(id)
		if noiseIDs[idLower] && !isStructuralID(idLower) {
			textLen := len(strings.TrimSpace(el.Text()))
			if textLen > 5000 {
				return false
			}
			return true
		}
		for _, prefix := range cookieConsentPrefixes {
			if strings.HasPrefix(idLower, prefix) {
				return true
			}
		}
	}

	// Cookie consent by class prefix
	if cls, exists := el.Attr("class"); exists {
		clsLower := strings.ToLower(cls)
		for _, prefix := range cookieConsentPrefixes {
			if strings.Contains(clsLower, prefix) {
				return true
			}
		}
	}

	return false
}

// isNoiseDescendant checks if an element is inside a noise container.
func isNoiseDescendant(el *goquery.Selection) bool {
	parent := el.Parent()
	for parent.Length() > 0 {
		if isNoise(parent) {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

// isNoiseClass checks if any class token matches noise patterns.
func isNoiseClass(class string) bool {
	for _, token := range strings.Fields(class) {
		if isNoiseToken(token) {
			return true
		}
	}
	return isAdClass(class)
}

// isNoiseToken checks a single class token against noise patterns.
func isNoiseToken(token string) bool {
	t := strings.ToLower(token)

	// Skip CSS variables and arbitrary values
	if strings.Contains(t, "[--") || strings.Contains(t, "var(") {
		return false
	}

	// Strip Tailwind responsive/state prefixes
	core := t
	if idx := strings.LastIndex(t, ":"); idx >= 0 {
		core = t[idx+1:]
	}

	// Skip Tailwind utility prefixes
	utilityPrefixes := []string{
		"p-", "pt-", "pb-", "pl-", "pr-", "px-", "py-",
		"m-", "mt-", "mb-", "ml-", "mr-", "mx-", "my-",
		"w-", "h-", "min-", "max-", "top-", "left-", "right-", "bottom-",
		"z-", "gap-", "text-", "bg-", "border-", "rounded-", "flex-", "grid-",
		"col-", "row-", "opacity-", "transition-", "duration-", "delay-", "ease-",
		"translate-", "scale-", "rotate-", "origin-", "overflow-", "inset-",
		"space-", "divide-", "ring-", "shadow-", "outline-", "font-", "leading-",
		"tracking-", "decoration-",
	}
	for _, pfx := range utilityPrefixes {
		if strings.HasPrefix(core, pfx) {
			return false
		}
	}

	// Prefix-only patterns
	for _, p := range []string{"banner", "overlay"} {
		if core == p || strings.HasPrefix(core, p+"-") || strings.HasPrefix(core, p+"_") {
			return true
		}
	}

	// Patterns <= 6 chars need word-boundary matching
	for _, p := range noiseClassPatterns {
		if len(p) <= 6 {
			if isWordBoundaryMatch(core, p) {
				return true
			}
		} else if strings.Contains(core, p) {
			return true
		}
	}

	return false
}

// isWordBoundaryMatch checks if pattern appears at word boundaries.
func isWordBoundaryMatch(text, pattern string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], pattern)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		beforeOK := abs == 0 || text[abs-1] == '-' || text[abs-1] == '_'
		end := abs + len(pattern)
		afterOK := end == len(text) || text[end] == '-' || text[end] == '_'
		if beforeOK && afterOK {
			return true
		}
		idx = abs + 1
	}
}

// isStructuralID checks if an ID is a structural wrapper.
func isStructuralID(id string) bool {
	for _, suffix := range structuralIDSuffixes {
		if strings.Contains(id, suffix) {
			return true
		}
	}
	return false
}

// isAdClass detects "ad" as a standalone class token.
func isAdClass(class string) bool {
	for _, token := range strings.Fields(class) {
		if token == "ad" || strings.HasPrefix(token, "ad-") || strings.HasPrefix(token, "ad_") ||
			strings.HasSuffix(token, "-ad") || strings.HasSuffix(token, "_ad") {
			return true
		}
	}
	return false
}
