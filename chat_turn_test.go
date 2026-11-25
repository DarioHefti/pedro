package main

import (
	"reflect"
	"testing"

	"pedro/shared"
)

func TestTurnMessagesToStore(t *testing.T) {
	call := func(json string) shared.Message { return shared.Message{Role: "assistant", ToolCalls: json} }
	result := func(id, content string) shared.Message {
		return shared.Message{Role: "tool", Content: content, ToolCallID: id}
	}
	reply := shared.Message{Role: "assistant", Content: "Done."}

	tests := []struct {
		name      string
		generated []shared.Message
		want      []shared.Message
	}{
		{
			name:      "plain reply",
			generated: []shared.Message{reply},
			want:      []shared.Message{reply},
		},
		{
			name: "tool rounds merge into one card before the reply",
			generated: []shared.Message{
				call(`[{"name":"tool_search","argsJSON":"{}","id":"a"}]`),
				result("a", "found web_search"),
				call(`[{"name":"web_search","argsJSON":"{\"query\":\"x\"}","id":"b"},{"name":"webclaw","argsJSON":"{}","id":"c"}]`),
				result("b", "results"),
				result("c", "page"),
				reply,
			},
			want: []shared.Message{
				call(`[{"name":"tool_search","argsJSON":"{}","id":"a"},{"name":"web_search","argsJSON":"{\"query\":\"x\"}","id":"b"},{"name":"webclaw","argsJSON":"{}","id":"c"}]`),
				result("a", "found web_search"),
				result("b", "results"),
				result("c", "page"),
				reply,
			},
		},
		{
			name: "trailing tool calls are kept",
			generated: []shared.Message{
				call(`[{"name":"grep","argsJSON":"{}","id":"g"}]`),
				result("g", "match"),
			},
			want: []shared.Message{
				call(`[{"name":"grep","argsJSON":"{}","id":"g"}]`),
				result("g", "match"),
			},
		},
		{
			name: "unparsable tool calls and their orphaned results are dropped",
			generated: []shared.Message{
				call(`not json`),
				result("x", "lost"),
				reply,
			},
			want: []shared.Message{reply},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := turnMessagesToStore(tt.generated); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestFinalReply(t *testing.T) {
	generated := []shared.Message{
		{Role: "assistant", ToolCalls: `[{"name":"grep"}]`},
		{Role: "tool", Content: "match"},
		{Role: "assistant", Content: "Here it is."},
	}
	if got := finalReply(generated); got != "Here it is." {
		t.Fatalf("got %q", got)
	}
	if got := finalReply(generated[:2]); got != "" {
		t.Fatalf("a turn without a reply should have none, got %q", got)
	}
}
