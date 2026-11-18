package browser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestBuildSnapshot(t *testing.T) {
	// A trimmed Accessibility.getFullAXTree response.
	raw := `[
	 {"nodeId":"1","ignored":false,"role":{"type":"role","value":"RootWebArea"},"name":{"type":"computedString","value":"Shop"},"childIds":["2","3","9"]},
	 {"nodeId":"2","ignored":true,"role":{"type":"role","value":"generic"},"childIds":["4"]},
	 {"nodeId":"4","ignored":false,"role":{"type":"role","value":"link"},"name":{"type":"computedString","value":"Home"},"backendDOMNodeId":11},
	 {"nodeId":"3","ignored":false,"role":{"type":"role","value":"form"},"name":{"type":"computedString","value":"Login"},"childIds":["5","6","7","8"]},
	 {"nodeId":"5","ignored":false,"role":{"type":"role","value":"heading"},"name":{"type":"computedString","value":"Sign in"},
	  "properties":[{"name":"level","value":{"type":"integer","value":2}}]},
	 {"nodeId":"6","ignored":false,"role":{"type":"role","value":"textbox"},"name":{"type":"computedString","value":""},"value":{"type":"string","value":"me@x.ch"},"backendDOMNodeId":12},
	 {"nodeId":"7","ignored":false,"role":{"type":"role","value":"checkbox"},"name":{"type":"computedString","value":"Remember me"},"backendDOMNodeId":13,
	  "properties":[{"name":"checked","value":{"type":"tristate","value":"true"}}]},
	 {"nodeId":"8","ignored":false,"role":{"type":"role","value":"button"},"name":{"type":"computedString","value":""},"backendDOMNodeId":14},
	 {"nodeId":"9","ignored":false,"role":{"type":"role","value":"button"},"name":{"type":"computedString","value":"Menu"},"backendDOMNodeId":15,
	  "properties":[{"name":"expanded","value":{"type":"booleanOrUndefined","value":false}}]}
	]`
	var nodes []*proto.AccessibilityAXNode
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		t.Fatal(err)
	}

	snap := buildSnapshot(nodes)
	want := strings.Join([]string{
		`- link "Home" [ref=e1]`,
		`- form "Login"`,
		`  - heading "Sign in" [level=2]`,
		`  - textbox value="me@x.ch" [ref=e2]`,
		`  - checkbox "Remember me" [checked] [ref=e3]`,
		`- button "Menu" [expanded=false] [ref=e4]`,
		``,
	}, "\n")
	if snap.text != want {
		t.Errorf("snapshot:\n%s\nwant:\n%s", snap.text, want)
	}
	if snap.refs["e2"] != 12 || snap.refs["e4"] != 15 || len(snap.refs) != 4 {
		t.Errorf("unexpected refs %v", snap.refs)
	}
}

func TestIsChallengeTitle(t *testing.T) {
	for _, title := range []string{"Just a moment...", "Verifying your request", "Attention Required! | Cloudflare"} {
		if !IsChallengeTitle(title) {
			t.Errorf("%q should be a challenge", title)
		}
	}
	if IsChallengeTitle("Rent law basics") {
		t.Error("normal title flagged as challenge")
	}
}
