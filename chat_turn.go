package main

import (
	"encoding/json"

	"pedro/shared"
)

// toolCallRecord is how a tool call is stored in a message's ToolCalls JSON; the frontend
// reads the same shape.
type toolCallRecord struct {
	Name     string `json:"name"`
	ArgsJSON string `json:"argsJSON"`
	ID       string `json:"id"`
}

// turnMessagesToStore turns the messages one chat turn generated (tool round-trips plus the
// final reply) into the rows to store, in order. Every tool-calling assistant message is merged
// into one, followed by all tool results, so the frontend shows a single "tools used" card
// per reply.
//
// Tool results are only kept together with at least one tool call, and tool calls whose JSON
// does not parse are dropped.
func turnMessagesToStore(generated []shared.Message) []shared.Message {
	var out []shared.Message
	var calls []toolCallRecord
	var results []shared.Message

	flush := func() {
		if len(calls) == 0 {
			return
		}
		merged, _ := json.Marshal(calls)
		out = append(out, shared.Message{Role: "assistant", ToolCalls: string(merged)})
		out = append(out, results...)
		calls, results = nil, nil
	}

	for _, m := range generated {
		switch {
		case m.Role == "assistant" && m.ToolCalls != "":
			var tcs []toolCallRecord
			if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err == nil {
				calls = append(calls, tcs...)
			}
		case m.Role == "tool":
			results = append(results, shared.Message{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID})
		default:
			flush()
			out = append(out, shared.Message{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID})
		}
	}
	// A turn should end with a reply, but keep trailing tool calls if it did not.
	flush()
	return out
}

// finalReply is the text of the last assistant message that is not a tool call.
func finalReply(generated []shared.Message) string {
	for i := len(generated) - 1; i >= 0; i-- {
		if generated[i].Role == "assistant" && generated[i].ToolCalls == "" {
			return generated[i].Content
		}
	}
	return ""
}
