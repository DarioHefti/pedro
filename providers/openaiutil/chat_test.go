package openaiutil

import (
	"encoding/json"
	"strings"
	"testing"

	"pedro/shared"
	"pedro/tools"
)

func TestFullSystemPrompt(t *testing.T) {
	base := "You are a bot."
	if got := FullSystemPrompt(base, "", ""); got != base {
		t.Fatalf("empty custom: got %q want %q", got, base)
	}
	custom := "Be brief."
	got := FullSystemPrompt(base, "", custom)
	if !strings.HasPrefix(got, base) {
		t.Fatalf("expected prefix %q", base)
	}
	if !strings.Contains(got, "## Additional Instructions") {
		t.Fatalf("expected additional section in %q", got)
	}
	if !strings.Contains(got, custom) {
		t.Fatalf("expected custom text in %q", got)
	}

	persona := "You are a pirate."
	got = FullSystemPrompt(base, persona, "")
	if !strings.Contains(got, "## Persona") {
		t.Fatalf("expected persona section in %q", got)
	}
	if !strings.Contains(got, persona) {
		t.Fatalf("expected persona text in %q", got)
	}

	got = FullSystemPrompt(base, persona, custom)
	if !strings.Contains(got, "## Persona") || !strings.Contains(got, "## Additional Instructions") {
		t.Fatalf("expected both persona and additional sections in %q", got)
	}
	personaIdx := strings.Index(got, "## Persona")
	customIdx := strings.Index(got, "## Additional Instructions")
	if personaIdx > customIdx {
		t.Fatalf("persona section should come before additional instructions")
	}
}

func TestBuildMessagesCountsAndOrder(t *testing.T) {
	sys := "system text"
	msgs := []shared.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "bye"},
	}
	out := BuildMessages(msgs, nil, sys)
	if len(out) != 1+len(msgs) {
		t.Fatalf("want %d messages (system + %d), got %d", 1+len(msgs), len(msgs), len(out))
	}
}

func TestBuildMessagesAttachesImagesOnlyToLastUserMessage(t *testing.T) {
	sys := "sys"
	msgs := []shared.Message{
		{Role: "user", Content: "first"},
		{Role: "user", Content: "last with image"},
	}
	images := []string{"data:image/png;base64,AAA"}
	out := BuildMessages(msgs, images, sys)
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
}

func TestToolDefinitionsNilRegistry(t *testing.T) {
	if ToolDefinitions(nil) != nil {
		t.Fatal("nil registry should yield nil tool defs")
	}
}

func TestToolDefinitionsFromRegistry(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(tools.NewSearchTool())
	r.Register(tools.NewFetchURLTool())
	defs := ToolDefinitions(r)
	if len(defs) < 2 {
		t.Fatalf("expected at least 2 tools, got %d", len(defs))
	}
	seen := map[string]bool{}
	for _, d := range defs {
		seen[d.Function.Name] = true
	}
	if !seen["web_search"] || !seen["fetch_url"] {
		t.Fatalf("missing expected tools: %+v", seen)
	}
}

func TestMaybeUnlockDirectTool(t *testing.T) {
	r := tools.New()
	unlocked := map[string]struct{}{}

	payload, err := json.Marshal(map[string]any{
		"action":    "describe",
		"tool_name": "read_file",
	})
	if err != nil {
		t.Fatal(err)
	}
	maybeUnlockDirectTool(string(payload), r, unlocked)
	if _, ok := unlocked["read_file"]; !ok {
		t.Fatalf("expected read_file to be unlocked, got %+v", unlocked)
	}
}

func TestMaybeUnlockDirectToolListUnlocksAll(t *testing.T) {
	r := tools.New()
	unlocked := map[string]struct{}{}

	payload, err := json.Marshal(map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatal(err)
	}
	maybeUnlockDirectTool(string(payload), r, unlocked)

	if len(unlocked) == 0 {
		t.Fatal("expected tools to be unlocked after list")
	}
	if _, ok := unlocked["show_file_tree"]; !ok {
		t.Fatalf("expected show_file_tree unlocked, got %+v", unlocked)
	}
	if _, ok := unlocked["tool_discovery"]; ok {
		t.Fatalf("tool_discovery must not be unlocked as a direct tool")
	}
}
