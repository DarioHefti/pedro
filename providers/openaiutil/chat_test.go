package openaiutil

import (
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

	seen := map[string]bool{}
	for _, d := range defs {
		seen[d.Function.Name] = true
	}

	if len(seen) != 0 {
		t.Fatalf("expected no immediate tools (all deferred), got %d: %+v", len(seen), seen)
	}

	deferred := r.DeferredDefinitions()
	if len(deferred) < 2 {
		t.Fatalf("expected at least 2 deferred tools, got %d", len(deferred))
	}
	deferredSeen := map[string]bool{}
	for _, d := range deferred {
		deferredSeen[d.Name] = true
	}
	if !deferredSeen["web_search"] || !deferredSeen["fetch_url"] {
		t.Fatalf("missing expected deferred tools: %+v", deferredSeen)
	}
}

func TestHandleToolSearchResult(t *testing.T) {
	unlocked := map[string]struct{}{}

	result := `{"tool_references":[{"tool_name":"read_file"},{"tool_name":"web_search"}]}`
	handleToolSearchResult(result, unlocked)

	if _, ok := unlocked["read_file"]; !ok {
		t.Fatal("expected read_file to be unlocked")
	}
	if _, ok := unlocked["web_search"]; !ok {
		t.Fatal("expected web_search to be unlocked")
	}
}

func TestHandleToolSearchResultInvalidJSON(t *testing.T) {
	unlocked := map[string]struct{}{}

	result := `not json`
	handleToolSearchResult(result, unlocked)

	if len(unlocked) != 0 {
		t.Fatalf("expected no tools unlocked for invalid JSON, got %+v", unlocked)
	}
}
