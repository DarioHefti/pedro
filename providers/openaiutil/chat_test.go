package openaiutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"pedro/shared"
	"pedro/tools"
)

func TestFullSystemPrompt(t *testing.T) {
	base := "You are a bot."
	got := FullSystemPrompt(base, "", "", time.Now())
	if !strings.HasPrefix(got, base) {
		t.Fatalf("expected prefix %q, got %q", base, got)
	}
	if !strings.Contains(got, "## Date and Time\nThis conversation started on ") {
		t.Fatalf("expected date and time section in %q", got)
	}
	if !strings.Contains(got, "## Operating System") {
		t.Fatalf("expected operating system section in %q", got)
	}
	custom := "Be brief."
	got = FullSystemPrompt(base, custom, "", time.Now())
	if !strings.HasPrefix(got, base) {
		t.Fatalf("expected prefix %q", base)
	}
	if !strings.Contains(got, "## Additional Instructions") {
		t.Fatalf("expected additional section in %q", got)
	}
	if !strings.Contains(got, custom) {
		t.Fatalf("expected custom text in %q", got)
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
	r.Register(tools.NewSearchTool(nil, nil))
	r.Register(tools.NewWebclawTool(nil, nil))
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
	if !deferredSeen["web_search"] || !deferredSeen["webclaw"] {
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

func TestIsToolReasoningConflict(t *testing.T) {
	azureErr := fmt.Errorf(`streaming error: POST "https://x.openai.azure.com/openai/chat/completions?api-version=2024-12-01-preview": 400 Bad Request { "message": "Function tools with reasoning_effort are not supported for gpt-6-luna in /v1/chat/completions. To use function tools, use /v1/responses or set reasoning_effort to 'none'.", "type": "invalid_request_error", "param": "reasoning_effort", "code": null }`)
	if !isToolReasoningConflict(azureErr) {
		t.Fatal("expected azure tools/reasoning error to be detected")
	}
	if isToolReasoningConflict(nil) {
		t.Fatal("nil error should not be a conflict")
	}
	if isToolReasoningConflict(fmt.Errorf("rate limit exceeded")) {
		t.Fatal("unrelated error should not be a conflict")
	}
}

func writeSSE(w http.ResponseWriter, payload string) {
	fmt.Fprintf(w, "data: %s\n\n", payload)
}

func TestStreamingChatRetriesWithReasoningNoneWhenToolsRejected(t *testing.T) {
	var (
		mu       sync.Mutex
		requests []map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()

		toolsParam, _ := body["tools"].([]any)
		if len(toolsParam) > 0 && body["reasoning_effort"] != "none" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Function tools with reasoning_effort are not supported for gpt-6-luna in /v1/chat/completions. To use function tools, use /v1/responses or set reasoning_effort to 'none'.","type":"invalid_request_error","param":"reasoning_effort","code":null}}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, `{"id":"1","object":"chat.completion.chunk","created":0,"model":"gpt-6-luna","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":null}]}`)
		writeSSE(w, `{"id":"1","object":"chat.completion.chunk","created":0,"model":"gpt-6-luna","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		writeSSE(w, `[DONE]`)
	}))
	defer server.Close()

	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"))
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))

	model := "gpt-6-luna-reasoning-retry-test"
	run := func() []shared.Message {
		messages, err := StreamingChat(
			context.Background(), client, model, registry,
			[]shared.Message{{Role: "user", Content: "hi"}}, nil, "sys",
			nil, nil, nil, nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return messages
	}

	first := run()
	if len(first) != 1 || first[0].Content != "ok" {
		t.Fatalf("first run messages: %+v", first)
	}

	mu.Lock()
	if len(requests) != 2 {
		mu.Unlock()
		t.Fatalf("expected rejected request + retry, got %d requests", len(requests))
	}
	retry := requests[1]
	mu.Unlock()

	if got := retry["reasoning_effort"]; got != "none" {
		t.Fatalf("retry reasoning_effort: got %v want none", got)
	}

	run()

	mu.Lock()
	total := len(requests)
	memoized := requests[total-1]
	mu.Unlock()

	if total != 3 {
		t.Fatalf("expected memoized request without rejection, got %d total requests", total)
	}
	if got := memoized["reasoning_effort"]; got != "none" {
		t.Fatalf("memoized reasoning_effort: got %v want none", got)
	}
}

func TestToolsCalledInHistory(t *testing.T) {
	history := []shared.Message{
		{Role: "user", Content: "make an infographic"},
		{Role: "assistant", ToolCalls: `[{"name":"tool_search","argsJSON":"{\"query\":\"image\"}","id":"s1"},{"name":"generate_image","argsJSON":"{}","id":"g1"}]`},
		// Searched but never called: must not stay loaded.
		{Role: "tool", ToolCallID: "s1", Content: `{"type":"tool_reference","tool_references":[{"tool_name":"generate_image"},{"tool_name":"browser"}]}`},
		{Role: "tool", ToolCallID: "g1", Content: "Image generated"},
		{Role: "assistant", Content: "done"},
	}
	called := toolsCalledInHistory(history)
	if len(called) != 1 {
		t.Fatalf("expected only generate_image, got %+v", called)
	}
	if _, ok := called["generate_image"]; !ok {
		t.Fatalf("expected generate_image to stay loaded, got %+v", called)
	}
}

func TestToolDefinitionsWithAddsOnlyDeferredTools(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(tools.NewToolSearchTool(r))
	r.Register(tools.NewGenerateImageTool(nil, nil))
	unlocked := map[string]struct{}{"tool_search": {}, "generate_image": {}, "unknown": {}}

	var names []string
	for _, d := range toolDefinitionsWith(r, unlocked) {
		names = append(names, d.Function.Name)
	}
	if strings.Join(names, ",") != "tool_search,generate_image" {
		t.Fatalf("tool list: got %v", names)
	}
}

func TestStreamingChatKeepsToolsUsedInEarlierTurns(t *testing.T) {
	var (
		mu        sync.Mutex
		toolNames []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		for _, tool := range body.Tools {
			toolNames = append(toolNames, tool.Function.Name)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, `{"id":"1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
		writeSSE(w, `[DONE]`)
	}))
	defer server.Close()

	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"))
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.Register(tools.NewGenerateImageTool(nil, nil))

	history := []shared.Message{
		{Role: "user", Content: "draw an infographic"},
		{Role: "assistant", ToolCalls: `[{"name":"generate_image","argsJSON":"{\"prompt\":\"x\"}","id":"g1"}]`},
		{Role: "tool", ToolCallID: "g1", Content: "Image generated and saved to: /tmp/x.png"},
		{Role: "assistant", Content: "here it is"},
		{Role: "user", Content: "fix the typo in the title"},
	}
	if _, err := StreamingChat(context.Background(), client, "keep-tools-test", registry, history, nil, "sys", nil, nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if strings.Join(toolNames, ",") != "tool_search,generate_image" {
		t.Fatalf("first request should already offer generate_image, got tools %v", toolNames)
	}
}

func TestWithoutToolSearchesDropsSearchCallsAndResults(t *testing.T) {
	history := []shared.Message{
		{Role: "user", Content: "draw it"},
		{Role: "assistant", ToolCalls: `[{"name":"tool_search","argsJSON":"{}","id":"s1"},{"name":"generate_image","argsJSON":"{}","id":"g1"}]`},
		{Role: "tool", ToolCallID: "s1", Content: `{"tool_references":[{"tool_name":"generate_image"}]}`},
		{Role: "tool", ToolCallID: "g1", Content: "Image generated"},
		{Role: "assistant", Content: "here"},
		{Role: "user", Content: "search only"},
		{Role: "assistant", ToolCalls: `[{"name":"tool_search","argsJSON":"{}","id":"s2"}]`},
		{Role: "tool", ToolCallID: "s2", Content: `{"tool_references":[]}`},
		{Role: "assistant", Content: "nothing found"},
	}
	out := withoutToolSearches(history)

	var got []string
	for _, m := range out {
		got = append(got, m.Role+":"+m.ToolCallID+":"+m.ToolCalls)
	}
	want := []string{
		"user::",
		`assistant::[{"name":"generate_image","argsJSON":"{}","id":"g1"}]`,
		"tool:g1:",
		"assistant::",
		"user::",
		"assistant::",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDateTimeSectionStaysFixedForTheChat(t *testing.T) {
	started := time.Date(2026, 11, 25, 8, 1, 2, 0, time.Local)
	first := dateTimeSection(started, started)
	later := dateTimeSection(started, started.Add(9*time.Hour))
	if first != later {
		t.Fatalf("the prompt changed within a day, which defeats prompt caching: %q vs %q", first, later)
	}
	if !strings.Contains(first, "Wednesday, 25 November 2026 at 08:01") || strings.Contains(first, "Today is") {
		t.Fatalf("section = %q", first)
	}

	nextDay := dateTimeSection(started, started.Add(24*time.Hour))
	if !strings.HasPrefix(nextDay, first) || !strings.HasSuffix(nextDay, " Today is Thursday, 26 November 2026.") {
		t.Fatalf("a chat continued the next day should also name today: %q", nextDay)
	}
	if dateTimeSection(started, started.Add(30*time.Hour)) != nextDay {
		t.Fatal("the prompt changed within the next day")
	}
}

// echoTool is a tool that returns its name, for driving the tool loop in tests.
type echoTool struct{ calls *int }

func (e echoTool) Definition() tools.Definition {
	return tools.Definition{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}}
}

func (e echoTool) Execute(string) (string, error) {
	*e.calls++
	return "echoed", nil
}

func TestStreamingChatStopsCallingToolsAtTheLimit(t *testing.T) {
	var (
		mu          sync.Mutex
		toolChoices []any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		toolChoices = append(toolChoices, body["tool_choice"])
		n := len(toolChoices)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		// A model that never stops calling tools, and ignores tool_choice "none": two
		// calls per request.
		writeSSE(w, fmt.Sprintf(`{"id":"1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"step ","tool_calls":[{"index":0,"id":"a%[1]d","type":"function","function":{"name":"echo","arguments":"{}"}},{"index":1,"id":"b%[1]d","type":"function","function":{"name":"echo","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, n))
		writeSSE(w, `[DONE]`)
	}))
	defer server.Close()

	calls := 0
	registry := tools.NewRegistry()
	registry.Register(echoTool{calls: &calls})
	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))

	generated, err := StreamingChat(context.Background(), client, "loop-test", registry,
		[]shared.Message{{Role: "user", Content: "loop forever"}}, nil, "sys", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != MaxToolCallsPerReply {
		t.Fatalf("ran %d tool calls, want the limit of %d", calls, MaxToolCallsPerReply)
	}
	mu.Lock()
	defer mu.Unlock()
	// 25 requests run two calls each, then one more request must answer without tools.
	if want := MaxToolCallsPerReply/2 + 1; len(toolChoices) != want {
		t.Fatalf("sent %d requests, want %d", len(toolChoices), want)
	}
	if toolChoices[0] != nil || toolChoices[len(toolChoices)-1] != "none" {
		t.Fatalf("tool_choice first=%v last=%v, want unset then none", toolChoices[0], toolChoices[len(toolChoices)-1])
	}
	last := generated[len(generated)-1]
	if last.Role != "assistant" || last.ToolCalls != "" || last.Content != "step " {
		t.Fatalf("turn should end with a text reply, got %+v", last)
	}
}

func TestStreamingChatKeepsTheToolLimitForTheWholeBatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		if requests > 1 {
			writeSSE(w, `{"id":"2","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
			writeSSE(w, `[DONE]`)
			return
		}
		var parts []string
		for i := 0; i < MaxToolCallsPerReply+3; i++ {
			parts = append(parts, fmt.Sprintf(`{"index":%d,"id":"c%d","type":"function","function":{"name":"echo","arguments":"{}"}}`, i, i))
		}
		writeSSE(w, `{"id":"1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[`+strings.Join(parts, ",")+`]},"finish_reason":"tool_calls"}]}`)
		writeSSE(w, `[DONE]`)
	}))
	defer server.Close()

	calls := 0
	registry := tools.NewRegistry()
	registry.Register(echoTool{calls: &calls})
	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))

	generated, err := StreamingChat(context.Background(), client, "batch-limit-test", registry,
		[]shared.Message{{Role: "user", Content: "many at once"}}, nil, "sys", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != MaxToolCallsPerReply {
		t.Fatalf("ran %d tool calls, want %d", calls, MaxToolCallsPerReply)
	}
	var notRun int
	for _, m := range generated {
		if m.Role == "tool" && m.Content == toolLimitResult {
			notRun++
		}
	}
	if notRun != 3 {
		t.Fatalf("%d calls got the limit result, want 3", notRun)
	}
}

func TestStreamingChatReturnsThePartialReplyWhenCancelled(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, `{"id":"1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Once upon "},"finish_reason":null}]}`)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	client := openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))
	onChunk := func(string) { cancel() }

	generated, err := StreamingChat(ctx, client, "cancel-test", nil,
		[]shared.Message{{Role: "user", Content: "tell me a story"}}, nil, "sys", onChunk, nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(generated) != 1 || generated[0].Role != "assistant" || generated[0].Content != "Once upon " {
		t.Fatalf("generated = %+v, want the partial reply", generated)
	}
}
