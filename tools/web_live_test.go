//go:build live

package tools

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// End-to-end smoke test of the web tools against real websites.
// Run with: go test -tags live -run Live -v ./tools/
func TestLiveWebTools(t *testing.T) {
	r := newTestRegistry()
	defer r.Close()

	run := func(tool, args string, mustContain ...string) string {
		t.Helper()
		start := time.Now()
		out := r.Execute(tool, args)
		t.Logf("%s %s (%s, %d chars)\n%s\n", tool, args, time.Since(start).Round(time.Millisecond), len(out), out[:min(len(out), 600)])
		for _, want := range mustContain {
			if !strings.Contains(out, want) {
				t.Errorf("%s output is missing %q", tool, want)
			}
		}
		return out
	}

	run("web_search", `{"query":"Kündigungsfrist Mietvertrag Schweiz"}`, "1. Title:", "Found by:")
	run("webclaw", `{"url":"https://www.fedlex.admin.ch/eli/cc/27/317_321_377/de"}`, "Art. 1", "offset=")
	run("web_find", `{"query":"Kündigungsfrist Probezeit","url":"https://www.fedlex.admin.ch/eli/cc/27/317_321_377/de"}`, "Art. 335 b", "sieben Tagen")
	run("site_search", `{"url":"https://gobyexample.com/","query":"closures anonymous functions"}`, "gobyexample.com/closures")

	nav := run("browser", `{"action":"navigate","url":"https://en.wikipedia.org/wiki/Main_Page"}`, `searchbox "Search Wikipedia"`)
	ref := regexp.MustCompile(`searchbox "Search Wikipedia".*\[ref=(e\d+)\]`).FindStringSubmatch(nav)
	if ref == nil {
		t.Fatal("no search box ref in snapshot")
	}
	run("browser", `{"action":"type","ref":"`+ref[1]+`","text":"Zurich","submit":true}`, "URL: https://en.wikipedia.org/wiki/Z")
	run("browser", `{"action":"read"}`, "Switzerland")
}
