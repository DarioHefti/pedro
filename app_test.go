package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"pedro/media"
	"pedro/shared"
	"pedro/tools"
)

// ---------------------------------------------------------------- test doubles

type emitted struct {
	name string
	data []any
}

// recordingEvents captures emitted events instead of sending them to a webview.
type recordingEvents struct {
	mu     sync.Mutex
	events []emitted
}

func (r *recordingEvents) Emit(name string, data ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, emitted{name, data})
}

func (r *recordingEvents) named(name string) []emitted {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []emitted
	for _, e := range r.events {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

// scriptedTurn is what the fake model does for one Chat call.
type scriptedTurn struct {
	toolCalls []toolCallRecord
	chunks    []string
	err       error
	// blockUntilCancelled streams the chunks, then waits for the context to end.
	blockUntilCancelled bool
}

// fakeLLM is an LLMClient that replays scripted turns, like a provider would stream them.
type fakeLLM struct {
	turns        []scriptedTurn
	seen         [][]shared.Message
	ctxs         []context.Context
	memory       string
	streamedOnce chan struct{}
}

func (f *fakeLLM) Chat(ctx context.Context, messages []shared.Message, _ []string, onChunk func(string), onToolCall func(name, argsJSON, id string), onRequestDone func(shared.RequestUsage), onRequestCaptured func(shared.CapturedRequest)) ([]shared.Message, error) {
	f.seen = append(f.seen, messages)
	f.ctxs = append(f.ctxs, ctx)
	if len(f.turns) == 0 {
		return nil, errors.New("fakeLLM: no scripted turn left")
	}
	turn := f.turns[0]
	f.turns = f.turns[1:]

	var generated []shared.Message
	if len(turn.toolCalls) > 0 {
		for _, tc := range turn.toolCalls {
			if onToolCall != nil {
				onToolCall(tc.Name, tc.ArgsJSON, tc.ID)
			}
		}
		raw, _ := json.Marshal(turn.toolCalls)
		generated = append(generated, shared.Message{Role: "assistant", ToolCalls: string(raw)})
		for _, tc := range turn.toolCalls {
			generated = append(generated, shared.Message{Role: "tool", Content: "result of " + tc.Name, ToolCallID: tc.ID})
		}
		if onRequestDone != nil {
			onRequestDone(shared.RequestUsage{PromptTokens: 100, CompletionTokens: 10})
		}
	}
	// Like a real provider, a failed turn returns what it generated before failing.
	if turn.err != nil {
		return generated, turn.err
	}
	var text strings.Builder
	for _, c := range turn.chunks {
		text.WriteString(c)
		onChunk(c)
	}
	if turn.blockUntilCancelled {
		if f.streamedOnce != nil {
			close(f.streamedOnce)
		}
		<-ctx.Done()
		return append(generated, shared.Message{Role: "assistant", Content: text.String()}), ctx.Err()
	}
	if onRequestCaptured != nil {
		body, _ := json.Marshal(map[string]any{"model": "fake-model", "messages": messages})
		onRequestCaptured(shared.CapturedRequest{RequestBody: string(body)})
	}
	if onRequestDone != nil {
		onRequestDone(shared.RequestUsage{PromptTokens: 200, CompletionTokens: 40})
	}
	return append(generated, shared.Message{Role: "assistant", Content: text.String()}), nil
}

func (f *fakeLLM) SetBaseSystemPrompt(string)   {}
func (f *fakeLLM) SetCustomSystemPrompt(string) {}
func (f *fakeLLM) SetMemoryContext(ctx string)  { f.memory = ctx }
func (f *fakeLLM) SignIn(context.Context) error { return nil }
func (f *fakeLLM) SignOut() error               { return nil }
func (f *fakeLLM) IsAuthenticated() bool        { return true }
func (f *fakeLLM) SetAuthenticated(bool)        {}
func (f *fakeLLM) Name() string                 { return "fake" }
func (f *fakeLLM) ExtractionClient() any        { return nil }
func (f *fakeLLM) ModelName() string            { return "fake-model" }

func newTestApp(t *testing.T, turns ...scriptedTurn) (*App, *fakeLLM, *recordingEvents) {
	t.Helper()
	llm := &fakeLLM{turns: turns}
	events := &recordingEvents{}
	dir := t.TempDir()
	app := &App{
		store:  openTestDatabase(t),
		llm:    llm,
		media:  media.NewSwitch(),
		images: media.NewDirImageStore(dir+"/generated", media.GeneratedImagesRoute),
		pasted: media.NewDirImageStore(dir+"/pasted", media.PastedImagesRoute),
		events: events,
	}
	return app, llm, events
}

func reply(chunks ...string) scriptedTurn { return scriptedTurn{chunks: chunks} }

type row struct{ role, content string }

func rows(t *testing.T, app *App, convID int64) []row {
	t.Helper()
	var out []row
	for _, m := range app.GetMessages(convID) {
		out = append(out, row{m.Role, m.Content})
	}
	return out
}

// ------------------------------------------------------------------------ tests

func TestApp_SendMessage_StreamsAndStoresTheTurn(t *testing.T) {
	app, llm, events := newTestApp(t, scriptedTurn{
		toolCalls: []toolCallRecord{
			{Name: "tool_search", ArgsJSON: `{"query":"web_search"}`, ID: "c1"},
			{Name: "web_search", ArgsJSON: `{"query":"wails"}`, ID: "c2"},
		},
		chunks: []string{"Wails ", "is ", "great."},
	})
	conv := app.CreateConversation()

	got := app.SendMessage(conv.ID, "What is Wails?", "")
	if got != "Wails is great." {
		t.Fatalf("reply = %q", got)
	}

	msgs := app.GetMessages(conv.ID)
	if len(msgs) != 5 {
		t.Fatalf("stored %d messages, want 5: %#v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "What is Wails?" {
		t.Errorf("first message = %#v", msgs[0])
	}
	var calls []toolCallRecord
	if err := json.Unmarshal([]byte(msgs[1].ToolCalls), &calls); err != nil || len(calls) != 2 {
		t.Errorf("merged tool calls = %q (%v)", msgs[1].ToolCalls, err)
	}
	if msgs[2].ToolCallID != "c1" || msgs[3].ToolCallID != "c2" || msgs[4].Content != "Wails is great." {
		t.Errorf("tool results / reply = %#v", msgs[2:])
	}

	if convs := app.GetConversations(); convs[0].Title != "What is Wails?" {
		t.Errorf("title = %q", convs[0].Title)
	}
	if n := len(events.named("stream_chunk")); n != 3 {
		t.Errorf("stream_chunk events = %d, want 3", n)
	}
	if n := len(events.named("tool_call")); n != 2 {
		t.Errorf("tool_call events = %d, want 2", n)
	}
	if n := len(events.named("conversation_updated")); n != 1 {
		t.Errorf("conversation_updated events = %d, want 1", n)
	}
	counts := app.GetRequestCounts(conv.ID)
	if counts.PerChat != 2 || counts.PerChatTokens != 350 || counts.Global != 2 {
		t.Errorf("request counts = %#v", counts)
	}
	if stats := app.GetLifetimeStats(); stats.TotalTokens != 350 {
		t.Errorf("lifetime stats = %#v", stats)
	}
	if len(llm.seen) != 1 || llm.seen[0][0].Content != "What is Wails?" {
		t.Errorf("model saw %#v", llm.seen)
	}

	details := app.GetLLMDetails()
	if len(details) != 1 || details[0].ConversationID != conv.ID || details[0].Model != "fake-model" {
		t.Fatalf("details = %#v", details)
	}
	var payload struct {
		Request  map[string]any `json:"request"`
		Response string         `json:"response"`
	}
	if err := json.Unmarshal([]byte(details[0].Messages), &payload); err != nil || payload.Response != "Wails is great." || payload.Request["model"] != "fake-model" {
		t.Errorf("captured payload = %s (%v)", details[0].Messages, err)
	}
}

func TestApp_SendMessage_ProviderErrorKeepsTheQuestion(t *testing.T) {
	app, _, _ := newTestApp(t, scriptedTurn{err: errors.New("429 rate limited")})
	conv := app.CreateConversation()

	if got := app.SendMessage(conv.ID, "Will this fail?", ""); got != "Error: 429 rate limited" {
		t.Fatalf("reply = %q", got)
	}
	if got := rows(t, app, conv.ID); len(got) != 1 || got[0] != (row{"user", "Will this fail?"}) {
		t.Fatalf("stored = %#v", got)
	}
}

func TestApp_SendMessage_WithoutProvider(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.llm = nil
	conv := app.CreateConversation()

	got := app.SendMessage(conv.ID, "Hello?", "")
	if !strings.Contains(got, "please configure LLM provider settings first") {
		t.Fatalf("reply = %q", got)
	}
	if n := len(app.GetMessages(conv.ID)); n != 1 {
		t.Fatalf("stored %d messages, want the question only", n)
	}
}

func TestApp_SendMessage_ToADeletedConversationFails(t *testing.T) {
	app, _, _ := newTestApp(t, reply("unused"))
	conv := app.CreateConversation()
	if err := app.DeleteConversation(conv.ID); err != nil {
		t.Fatal(err)
	}
	if got := app.SendMessage(conv.ID, "Anyone there?", ""); !strings.HasPrefix(got, "Error: Failed to save message") {
		t.Fatalf("reply = %q", got)
	}
}

func TestApp_AbortMessage_KeepsThePartialReply(t *testing.T) {
	streamed := make(chan struct{})
	app, llm, _ := newTestApp(t, scriptedTurn{
		toolCalls:           []toolCallRecord{{Name: "web_search", ArgsJSON: `{"query":"story"}`, ID: "c1"}},
		chunks:              []string{"Once ", "upon "},
		blockUntilCancelled: true,
	})
	llm.streamedOnce = streamed
	conv := app.CreateConversation()

	done := make(chan string, 1)
	go func() { done <- app.SendMessage(conv.ID, "Tell me a story", "") }()

	select {
	case <-streamed:
	case <-time.After(5 * time.Second):
		t.Fatal("the reply never started streaming")
	}
	app.AbortMessage()

	select {
	case got := <-done:
		if got != "Once upon " {
			t.Fatalf("partial reply = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("abort did not end the reply")
	}
	msgs := app.GetMessages(conv.ID)
	if len(msgs) != 4 {
		t.Fatalf("stored %d messages, want question, tool call, tool result, partial reply: %#v", len(msgs), msgs)
	}
	if msgs[1].ToolCalls == "" || msgs[2].ToolCallID != "c1" || msgs[3].Role != "assistant" || msgs[3].Content != "Once upon " {
		t.Fatalf("stored = %#v", msgs)
	}
}

func TestApp_SendMessage_ProviderErrorKeepsTheToolsThatRan(t *testing.T) {
	app, _, _ := newTestApp(t, scriptedTurn{
		toolCalls: []toolCallRecord{{Name: "web_search", ArgsJSON: `{"query":"x"}`, ID: "c1"}},
		err:       errors.New("connection reset"),
	})
	conv := app.CreateConversation()

	if got := app.SendMessage(conv.ID, "Search for x", ""); got != "Error: connection reset" {
		t.Fatalf("reply = %q", got)
	}
	msgs := app.GetMessages(conv.ID)
	if len(msgs) != 3 || msgs[1].ToolCalls == "" || msgs[2].ToolCallID != "c1" {
		t.Fatalf("stored = %#v, want the question and the tool round-trip", msgs)
	}
}

func TestApp_PromptTimeIsWhenTheChatStarted(t *testing.T) {
	app, llm, _ := newTestApp(t, reply("one"), reply("two"))
	conv := app.CreateConversation()
	before := time.Now().Add(-2 * time.Second)
	app.SendMessage(conv.ID, "first", "")
	app.SendMessage(conv.ID, "second", "")

	first, second := shared.ChatStart(llm.ctxs[0]), shared.ChatStart(llm.ctxs[1])
	if !first.Equal(second) {
		t.Fatalf("turns got different start times %v and %v; the prompt would change every turn", first, second)
	}
	// Stored in UTC by SQLite: it must still read as the actual moment, not shifted by the zone.
	if first.Before(before) || first.After(time.Now().Add(2*time.Second)) {
		t.Fatalf("start time %v is not when the chat started (around %v)", first, before)
	}
	if want := app.GetMessages(conv.ID)[0].CreatedAt; !first.Equal(want) {
		t.Fatalf("start time %v, want the first message's %v", first, want)
	}
}

func TestApp_FileToolsReadOnlyWhatTheChatWasGiven(t *testing.T) {
	dir := t.TempDir()
	attached := filepath.Join(dir, "attached.txt")
	other := filepath.Join(dir, "other.txt")
	for _, p := range []string{attached, other} {
		if err := os.WriteFile(p, []byte("contents of "+filepath.Base(p)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app, llm, _ := newTestApp(t, reply("ok"))
	conv := app.CreateConversation()
	atts, _ := json.Marshal([]map[string]string{{"type": "file-ref", "content": attached, "name": "attached.txt"}})
	app.SendMessage(conv.ID, "Summarize this", string(atts))

	read := func(path string) string {
		out, _ := tools.NewReadFileTool().ExecuteContext(llm.ctxs[0], `{"path":`+strconv.Quote(path)+`}`)
		return out
	}
	if out := read(attached); !strings.Contains(out, "contents of attached.txt") {
		t.Errorf("attached file: %q", out)
	}
	if out := read(other); !strings.Contains(out, "access denied") {
		t.Errorf("a file that was not attached was readable: %q", out)
	}
}

func seedTwoTurns(t *testing.T, app *App) int64 {
	t.Helper()
	conv := app.CreateConversation()
	for _, r := range []row{{"user", "q1"}, {"assistant", "a1"}, {"user", "q2"}, {"assistant", "a2"}} {
		if _, err := app.store.AddMessage(conv.ID, r.role, r.content, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	return conv.ID
}

func TestApp_ResendMessage_TruncatesAfterTheQuestion(t *testing.T) {
	app, llm, _ := newTestApp(t, reply("a1 again"))
	id := seedTwoTurns(t, app)

	if got := app.ResendMessage(id, 0); got != "a1 again" {
		t.Fatalf("reply = %q", got)
	}
	want := []row{{"user", "q1"}, {"assistant", "a1 again"}}
	if got := rows(t, app, id); !equalRows(got, want) {
		t.Fatalf("stored = %#v", got)
	}
	if len(llm.seen[0]) != 1 {
		t.Fatalf("model saw %d messages, want only q1", len(llm.seen[0]))
	}
}

func TestApp_ResendMessage_TheLastQuestion(t *testing.T) {
	app, _, _ := newTestApp(t, reply("answer"))
	conv := app.CreateConversation()
	app.store.AddMessage(conv.ID, "user", "only question", "", "", "")

	if got := app.ResendMessage(conv.ID, 0); got != "answer" {
		t.Fatalf("reply = %q", got)
	}
}

func TestApp_ResendMessage_RejectsBadIndexes(t *testing.T) {
	app, _, _ := newTestApp(t)
	id := seedTwoTurns(t, app)
	if got := app.ResendMessage(id, 9); got != "Error: Invalid message index" {
		t.Errorf("out of range: %q", got)
	}
	if got := app.ResendMessage(id, 1); got != "Error: No user message to resend" {
		t.Errorf("assistant index: %q", got)
	}
	if n := len(app.GetMessages(id)); n != 4 {
		t.Errorf("rejected resend changed the thread: %d messages", n)
	}
}

func TestApp_RegenerateMessage_ReplacesFromTheReply(t *testing.T) {
	app, llm, _ := newTestApp(t, reply("a2 rewritten"))
	id := seedTwoTurns(t, app)

	if got := app.RegenerateMessage(id, 3); got != "a2 rewritten" {
		t.Fatalf("reply = %q", got)
	}
	want := []row{{"user", "q1"}, {"assistant", "a1"}, {"user", "q2"}, {"assistant", "a2 rewritten"}}
	if got := rows(t, app, id); !equalRows(got, want) {
		t.Fatalf("stored = %#v", got)
	}
	if len(llm.seen[0]) != 3 {
		t.Fatalf("model saw %d messages, want the 3 before the reply", len(llm.seen[0]))
	}
	if got := app.RegenerateMessage(id, 0); got != "Error: No assistant message to regenerate" {
		t.Errorf("user index: %q", got)
	}
}

func TestApp_DeleteMessagesFrom(t *testing.T) {
	app, _, _ := newTestApp(t)
	id := seedTwoTurns(t, app)
	if err := app.DeleteMessagesFrom(id, 2); err != nil {
		t.Fatal(err)
	}
	if got := rows(t, app, id); !equalRows(got, []row{{"user", "q1"}, {"assistant", "a1"}}) {
		t.Fatalf("stored = %#v", got)
	}
}

func TestApp_MemoriesReachTheModel(t *testing.T) {
	app, llm, _ := newTestApp(t, reply("ok"))
	if err := app.SaveMemory("home_city", "Zurich", "personal"); err != nil {
		t.Fatal(err)
	}
	conv := app.CreateConversation()
	app.SendMessage(conv.ID, "Weather?", "")
	if !strings.Contains(llm.memory, "home_city: Zurich") {
		t.Fatalf("memory context = %q", llm.memory)
	}
}

func TestApp_MemoriesSection_ListsCategoriesByImportanceAndIsStable(t *testing.T) {
	app, _, _ := newTestApp(t)
	for _, m := range []struct {
		key, category string
		importance    int
	}{
		{"hobby", "personal", 2},
		{"editor", "tools", 3},
		{"allergy", "health", 5},
		{"city", "personal", 4},
	} {
		if err := app.store.SaveMemoryWithMeta(m.key, "x", m.category, "test", m.importance); err != nil {
			t.Fatal(err)
		}
	}

	first := app.memoriesSection()
	health, personal, tools := strings.Index(first, "### health"), strings.Index(first, "### personal"), strings.Index(first, "### tools")
	if health < 0 || !(health < personal && personal < tools) {
		t.Fatalf("categories out of importance order:\n%s", first)
	}
	if !strings.Contains(first, "- allergy: x [critical]") {
		t.Fatalf("critical memory missing:\n%s", first)
	}
	for i := 0; i < 20; i++ {
		if again := app.memoriesSection(); again != first {
			t.Fatalf("memory section changed between calls:\n%s\n---\n%s", first, again)
		}
	}
}

func TestApp_DeleteConversation_RemovesItsMessagesAndDetails(t *testing.T) {
	app, _, _ := newTestApp(t, reply("hi"))
	conv := app.CreateConversation()
	app.SendMessage(conv.ID, "hello", "")
	if err := app.DeleteConversation(conv.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(app.GetMessages(conv.ID)); n != 0 {
		t.Errorf("%d messages left", n)
	}
	if n := len(app.GetLLMDetails()); n != 0 {
		t.Errorf("%d request details left", n)
	}
}

func equalRows(a, b []row) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
