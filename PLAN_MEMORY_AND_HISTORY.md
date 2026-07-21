# Pedro: Memory Injection & Chat History Reliability — Implementation Plan

## Context

Two systemic issues degrade the assistant's reliability:

1. **The LLM has to search for its own memories.** When a user asks a question, the system only injects memories that match keywords from the query. If the keywords don't align (e.g. "motorbike" vs. memory key `owns_vehicle`), the LLM gets no relevant context and either guesses or wastefully calls `memory_search`.

2. **Tool-call rounds are dropped from chat history.** When the LLM uses tools, the intermediate assistant message (with `tool_calls`) and the tool-result messages are never persisted. `BuildMessages()` only understands `user` and `assistant` roles. On the next turn, the LLM sees a sanitized history that omits its own tool usage, leading to:
   - Duplicate tool calls
   - Loss of context from previous tool results
   - Confusion about what happened in prior turns

---

## Issue 1: Memory Injection

### Current Behaviour

`app.go` builds a memory context with two sections:

- `## Relevant Memories` — FTS5/LIKE keyword search on the current query (top 5)
- `## Available Memory Keys` — list of top 20 memory keys, instructing the LLM to call `memory_search` for details

This fails when:
- Keyword extraction doesn't match the memory key/value (`"motorbike"` doesn't match `owns_vehicle`)
- The LLM decides to call `memory_search` instead of using pre-injected facts, burning a tool roundtrip

### Research: How High-Star Projects Handle Memory

| Project | Stars | Approach |
|---------|-------|----------|
| **mem0** | 61.4k | Always retrieves relevant memories via embedding + BM25 + entity linking and injects them into the system prompt. Uses single-pass extraction. Never asks the agent to "search its own memory". |
| **LangChain** | 142k | `VectorStoreRetrieverMemory` retrieves relevant docs and prepends them. `ConversationBufferMemory` buffers raw history. Memory is the system's job, not the model's. |

**Key insight from both:** The runtime is responsible for memory retrieval. The LLM should never need to call a tool to read memories that belong in its context window.

### Target Behaviour

- All durable memories are automatically available to the LLM without a tool call.
- `memory_search` is removed as a runtime concern for the LLM (can stay as a user-facing settings feature).
- The system prompt is updated so the LLM knows its memories are pre-loaded.

### Implementation Steps

1. **Stop telling the LLM to search for memories**
   - Remove `## Available Memory Keys` and the instruction "Call `memory_search` with a key to retrieve its full value" from `shared/types.go` (`DefaultSystemPrompt`).
   - Delete `memoryKeysSection()` in `app.go`.

2. **Replace keyword-only retrieval with importance-first injection**
   - In `relevantMemoriesSection`, retrieve memories sorted by importance (highest first), not by keyword match.
   - Inject up to a fixed token budget (e.g. top 30 memories or ~2k tokens). For typical personal use this is all memories.
   - Keep the keyword/BM25 search only as a fallback for very large memory stores, but the default path should be "inject the most important memories regardless of query".
   - **New (added):** Implement a lightweight token estimator in `app.go` (e.g. `len(memoryString)/4` as a rough token count) so we can set a hard token ceiling (e.g. 1500 tokens) and truncate the lowest-importance memories if the user has many. This prevents context-window overflow.

3. **Remove the implicit need for `memory_search` as a tool**
   - Since critical memories are always injected, the LLM should not need `memory_search`.
   - In `tools/tool.go`, change line 189 from `r.Register(NewMemorySearchTool(backend))` to simply not register it. If you want to keep it executable for `tool_discovery` backward compat, use `r.RegisterHidden(NewMemorySearchTool(backend))` instead.
   - The frontend Settings UI already calls `GetMemories()` directly; it does not need the tool.

4. **(Optional future) Semantic retrieval**
   - If memory stores grow large, add an `embedding` BLOB column to the `memories` table and use a lightweight Go vector similarity search. This is out of scope for the immediate fix.

---

## Issue 2: Chat History Reliability

### Current Behaviour

The `messages` table only stores:

```
role: user | assistant
content: text
tool_calls: JSON of tool calls (for assistant final response only)
```

When a tool roundtrip happens inside `StreamingChat` (`providers/openaiutil/chat.go`):

```go
apiMessages = append(apiMessages, msg.ToParam())           // assistant with tool_calls — NOT SAVED
for _, tc := range msg.ToolCalls {
    result := registry.Execute(...)
    apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID)) // tool result — NOT SAVED
}
```

Only the final assistant text is stored via `AddMessage(..., "assistant", resp, "", toolCallsJSON)`.

On the next user message, `BuildMessages` reconstructs the OpenAI params from DB history:

```go
for _, m := range messages {
    switch m.Role {
    case "user":    // OK
    case "assistant": // OK (final text only)
    // tool and tool_call are IGNORED
    }
}
```

**Result:** The LLM on turn N has no record of tool calls it issued on turn N−1.

### Target Behaviour

The exact OpenAI message sequence is persisted and replayed, including:

1. Assistant message with `tool_calls` (content usually empty)
2. One `tool` message per tool call result (with `tool_call_id`)
3. Final assistant message with text content

### Implementation Steps

1. **Extend DB schema for tool-role messages**
   - Add `tool_call_id TEXT NOT NULL DEFAULT ''` column to `messages`.
   - Update `Database.migrate()` with:
     ```sql
     ALTER TABLE messages ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT ''
     ```
   - **New (added):** No new table needed; we store the full OpenAI sequence inside the existing `messages` table using roles: `user`, `assistant`, `tool`.

2. **Extend data model**
   - Update `Message` struct in `interfaces.go` to include `ToolCallID string`.
   - Update `shared.Message` in `shared/types.go` to include `ToolCallID string`.
   - **New (added):** Ensure the struct has `json:"tool_call_id,omitempty"` tags so `CaptureMessages` can serialize them correctly for `llm_history`.

3. **Persist full tool roundtrips**
   - Modify `runChat` in `app.go` (or `StreamingChat`) so that after a streaming turn completes, all intermediate messages are saved.
   - Easiest approach: change `StreamingChat` to accept an `onIntermediateMessage` callback, or return a slice of all messages generated during the turn.
   - Recommended minimal change: have `StreamingChat` return `[]shared.Message` representing the full delta of messages it generated (assistant-with-tool-calls, tool-results, final-assistant). `app.go` then persists each one with the correct role.
   - Alternative: inside `StreamingChat`, call a provided callback after each assistant-tool-result round:
     ```go
     onHistoryUpdate(role, content, toolCallID string)
     ```
     This keeps the DB code out of `openaiutil`.
   - **New (added — critical detail):** The OpenAI `msg.ToParam()` produced when the assistant issues tool calls must be saved as an `assistant` row with its `ToolCalls` JSON in the existing `ToolCalls` column and `content` empty (or whatever reasoning text OpenAI provided). Each subsequent `openai.ToolMessage(result, tc.ID)` must be saved as `role="tool"`, `content=result`, `tool_call_id=tc.ID`.
   - **New (added):** The final assistant text (after all tool roundtrips) is saved as `role="assistant"`, `content=resp`, with `tool_calls=''` (empty), because this final message does not contain tool calls.

4. **Update message assembly**
   - Rewrite `BuildMessages` in `providers/openaiutil/chat.go` to handle:
     - `role == "assistant"` + `tool_calls != ""` → `openai.AssistantMessage` with `ToolCalls` unmarshalled from JSON. **Content is usually empty for these rows.**
     - `role == "tool"` → `openai.ToolMessage(content, toolCallID)`
   - Ensure `CaptureMessages` uses `tool_call_id` when capturing tool messages.
   - **New (added):** `BuildMessages` must also preserve the standard case `role == "assistant"` + `tool_calls == ""` → `openai.AssistantMessage(m.Content)` (final text response).

5. **Ensure tool call IDs survive the roundtrip**
   - When the assistant message with `tool_calls` is saved, the JSON must contain the OpenAI-generated `id` field for each tool call (e.g. `"call_abc123"`).
   - The subsequent `tool` rows must reference the exact same IDs in their `tool_call_id` column.
   - **New (added):** This means `toolCallRecord` in `app.go` (or a new struct) must capture both `Name`, `ArgsJSON` **and** `ID` from OpenAI's `tc.ID` field.

6. **Handle message deletion with interleaved tool messages**
   - **New (added):** When a user deletes a message at index `i`, the current code calls `DeleteMessage(conversationID, i)` which deletes by ordinal offset. If tool messages are interleaved, a user might try to delete a `user` message but leave orphaned `assistant`+`tool` messages behind.
   - Solution: `DeleteMessage` should accept a range or `DeleteMessagesAfter(conversationID, messageIndex)` should be used. In `app.go`, `deleteMessage`, `resendMessage`, and `regenerateMessage` already iterate and delete subsequent messages; ensure this loop also deletes any interleaved `tool` rows.

7. **Interface change for `shared.LLMClient`**
   - **New (added):** If `StreamingChat` returns a slice of generated messages, the `Chat` method signature in `shared.LLMClient` must change from returning only `error` to returning `([]shared.Message, error)` or accepting a callback.
   - All providers (`openai`, `azure`, `azure_apikey`) use the same `StreamingChat` wrapper, so only the interface in `shared/types.go` and the provider wrappers need updating.

8. **Frontend / Wails compatibility**
   - Update `frontend/src/services/wailsService.ts` models (auto-generated via Wails, but at least ensure the new field is in the Go struct tags so Wails generates it).
   - **New (added):** After modifying Go structs, run `wails generate module` (or `wails dev`) so the TypeScript models in `wailsjs/go/models.ts` are regenerated.
   - `frontend/src/hooks/useMessaging.ts` `buildToolCallMaps` should still work since tool calls are still stored on the intermediate assistant message (index N) while the final assistant text is at index N+2.
   - **New (added):** The frontend currently renders tool calls from `messageToolCalls` which maps message index → tool calls. If the intermediate assistant message now appears as a distinct row in `messages`, the UI must render its tool call summary correctly. `Chat.tsx` currently attaches tool call summaries to `assistant` messages by looking up `messageToolCalls.get(i)`. This should work unchanged if we store the tool_calls on the intermediate assistant row and `buildToolCallMaps` maps that row's index correctly.

---

## Phasing

| Phase | Scope | Files Touched |
|-------|-------|---------------|
| **1** | Memory injection fix | `shared/types.go`, `app.go`, `tools/tool.go` |
| **2** | Schema + model extension | `database.go`, `interfaces.go`, `shared/types.go` |
| **3** | Persist tool roundtrips (StreamingChat signature + app.go plumbing) | `app.go`, `providers/openaiutil/chat.go`, `shared/types.go` |
| **4** | Replay tool roundtrips in BuildMessages | `providers/openaiutil/chat.go` |
| **5** | Handle edge cases (deletion, resend, regenerate) | `app.go`, `database.go` |
| **6** | Regenerate Wails bindings + validate frontend | `wailsjs/go/models.ts`, `frontend/src/hooks/useMessaging.ts` |
| **7** | Tests | `providers/openaiutil/chat_test.go`, `app.go` integration tests |

---

## Files to Examine / Modify

1. `app.go` — `buildMemoryContext`, `sendMessage`, `runChat`, `relevantMemoriesSection`, `memoryKeysSection`, `DeleteMessage`, `ResendMessage`, `RegenerateMessage`
2. `shared/types.go` — `DefaultSystemPrompt`, `Message`, `LLMClient`
3. `interfaces.go` — `Message` struct
4. `database.go` — `init`, `migrate`, `AddMessage`, `GetMessages`
5. `providers/openaiutil/chat.go` — `BuildMessages`, `StreamingChat`, `CaptureMessages`
6. `tools/tool.go` — Memory tool registry registration
7. `frontend/src/hooks/useMessaging.ts` — ensure `buildToolCallMaps` still works after schema change

---

## Testing Checklist (added)

- [ ] `BuildMessages` reconstructs `user`, `assistant`, `assistant`+`tool_calls`, and `tool` roles into valid OpenAI params.
- [ ] `StreamingChat` roundtrip: assistant issues tool call → tool result → final text. All three are persisted and replayable.
- [ ] Deleting a user message also removes subsequent assistant+tool messages from that turn.
- [ ] Resend and regenerate correctly truncate messages that include interleaved tool rows.
- [ ] Memory injection: "What motorbike do I have?" finds `owns_vehicle` without a `memory_search` tool call.
- [ ] `memory_search` tool is no longer visible to the model (not in `ToolDefinitions` output).
- [ ] Wails bindings compile and the frontend loads messages without JS errors.

---

## Open Questions

1. Should `memory_search` be completely deleted as a tool, or only hidden from the LLM registry?
   - **Recommendation:** Remove from the LLM tool registry. The frontend Settings UI can still call `SearchMemories` directly via the backend for user-driven search.

2. How do we want to bound the size of injected memories (token budget vs. count cap)?
   - **Recommendation:** Cap by count (top 30 by importance) for now. Most users will have <30 memories, so this is effectively "inject all". Later, switch to token-aware truncation.

3. If `StreamingChat`'s return signature changes, do all providers need updates?
   - **Recommendation:** Yes, but Pedro's providers (`openai`, `azure`, etc.) all share the single `StreamingChat` implementation in `openaiutil`. Changing the signature once covers all of them.
