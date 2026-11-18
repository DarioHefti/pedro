package fetch

import (
	"fmt"
	"runtime"
)

// Profile is the browser identity presented by the impersonating client.
// All fields must describe the same browser build, otherwise bot detectors
// flag the mismatch between headers and the TLS/HTTP2 fingerprint.
type Profile struct {
	ChromeMajor    int
	UserAgent      string
	SecCHUA        string
	Platform       string // value of sec-ch-ua-platform, e.g. `"macOS"`
	AcceptLanguage string
}

// chromeMajor is the Chrome version we claim. The uTLS ClientHello used by the
// transport (HelloChrome_133) matches every Chrome release since 133.
const chromeMajor = 145

// ChromeProfile returns a Chrome profile whose OS matches the machine Pedro
// runs on, so the claimed platform agrees with the real TCP/IP fingerprint.
func ChromeProfile() Profile {
	osToken, platform := platformTokens(runtime.GOOS)
	return Profile{
		ChromeMajor: chromeMajor,
		UserAgent: fmt.Sprintf(
			"Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36",
			osToken, chromeMajor),
		SecCHUA: fmt.Sprintf(
			`"Not:A-Brand";v="99", "Google Chrome";v="%d", "Chromium";v="%d"`,
			chromeMajor, chromeMajor),
		Platform:       platform,
		AcceptLanguage: "en-US,en;q=0.9",
	}
}

func platformTokens(goos string) (osToken, platform string) {
	switch goos {
	case "darwin":
		return "Macintosh; Intel Mac OS X 10_15_7", `"macOS"`
	case "linux":
		return "X11; Linux x86_64", `"Linux"`
	default:
		return "Windows NT 10.0; Win64; x64", `"Windows"`
	}
}
