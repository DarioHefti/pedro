package openaiutil

import (
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go"
	"pedro/shared"
	"pedro/tools"
)

// reasoningEffortNone disables reasoning. Some reasoning models (e.g. gpt-5/gpt-6
// on Azure) reject function tools on the Chat Completions API unless reasoning
// is turned off with this exact value.
const reasoningEffortNone = openai.ReasoningEffort("none")

// modelsRequiringReasoningNone remembers models that rejected function tools
// because of a non-none reasoning effort, so follow-up requests skip straight
// to reasoning_effort="none" instead of failing once per request.
var modelsRequiringReasoningNone sync.Map

func requiresReasoningNone(model string) bool {
	_, ok := modelsRequiringReasoningNone.Load(model)
	return ok
}

func markRequiresReasoningNone(model string) {
	modelsRequiringReasoningNone.Store(model, struct{}{})
}

// isToolReasoningConflict reports whether err is the provider rejecting
// function tools combined with reasoning effort on the chat completions API.
func isToolReasoningConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "reasoning_effort") {
		return false
	}
	return strings.Contains(msg, "function tools")
}

func userFacingOSName() string {
	switch goruntime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return goruntime.GOOS
	}
}

func userFacingShellHint() string {
	switch goruntime.GOOS {
	case "windows":
		return "PowerShell (not bash)"
	default:
		return "bash or zsh"
	}
}

// dateTimeSection tells the model when the conversation started. It is fixed for the whole
// chat on purpose: the system prompt is the start of every request and providers cache prompts
// by exact prefix, so a clock here would make every turn miss the cache. Only when the chat
// goes on past the day it started is today's date added, which changes the prompt at most
// once a day.
func dateTimeSection(started, now time.Time) string {
	started, now = started.In(time.Local), now.In(time.Local)
	out := fmt.Sprintf("This conversation started on %s at %s (time zone %s, UTC%s); the current time may be later.",
		started.Format("Monday, 2 January 2006"), started.Format("15:04"), started.Format("MST"), started.Format("-07:00"))
	if sy, sm, sd := started.Date(); !(now.Year() == sy && now.Month() == sm && now.Day() == sd) {
		out += " Today is " + now.Format("Monday, 2 January 2006") + "."
	}
	return out
}

// FullSystemPrompt builds the final system prompt from the base instructions,
// optional custom instructions, and optional memory context. started is when the
// conversation began (see dateTimeSection): it keeps the prompt identical across the turns
// of a chat, so providers can reuse their prompt cache.
func FullSystemPrompt(base, custom, memoryCtx string, started time.Time) string {
	out := base + "\n\n## Date and Time\n" + dateTimeSection(started, time.Now())
	out += fmt.Sprintf(
		"\n\n## Operating System\nThe user is running Pedro on %s (%s/%s). When suggesting terminal commands, file paths, keyboard shortcuts, or other OS-specific steps, use conventions appropriate for this operating system. Default shell: %s.",
		userFacingOSName(),
		goruntime.GOOS,
		goruntime.GOARCH,
		userFacingShellHint(),
	)
	if custom != "" {
		out += "\n\n## Additional Instructions\n" + custom
	}
	if memoryCtx != "" {
		out += "\n\n" + memoryCtx
	}
	return out
}

// ToolDefinitions converts the tool registry into OpenAI-compatible params.
func ToolDefinitions(registry *tools.Registry) []openai.ChatCompletionToolParam {
	if registry == nil {
		return nil
	}
	return toOpenAIToolDefinitions(registry.Definitions())
}

func toOpenAIToolDefinitions(defs []tools.Definition) []openai.ChatCompletionToolParam {
	var result []openai.ChatCompletionToolParam
	for _, def := range defs {
		result = append(result, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        def.Name,
				Description: openai.String(def.Description),
				Parameters:  openai.FunctionParameters(def.Parameters),
			},
		})
	}
	return result
}

type toolCallRecord struct {
	Name     string `json:"name"`
	ArgsJSON string `json:"argsJSON"`
	ID       string `json:"id"`
}

// BuildMessages converts shared messages + images into OpenAI message params.
func BuildMessages(messages []shared.Message, imageDataURLs []string, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	result := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	totalUsers := 0
	for _, m := range messages {
		if m.Role == "user" {
			totalUsers++
		}
	}

	userCount := 0
	for _, m := range messages {
		switch m.Role {
		case "user":
			userCount++
			isLastUser := userCount == totalUsers
			if isLastUser && len(imageDataURLs) > 0 {
				parts := []openai.ChatCompletionContentPartUnionParam{
					openai.TextContentPart(m.Content),
				}
				for _, img := range imageDataURLs {
					parts = append(parts, openai.ImageContentPart(
						openai.ChatCompletionContentPartImageImageURLParam{
							URL:    img,
							Detail: "auto",
						},
					))
				}
				result = append(result, openai.UserMessage(parts))
			} else {
				result = append(result, openai.UserMessage(m.Content))
			}
		case "assistant":
			if m.ToolCalls != "" {
				param := openai.AssistantMessage(m.Content)
				var tcs []toolCallRecord
				if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err == nil && len(tcs) > 0 {
					valid := true
					var toolCallParams []openai.ChatCompletionMessageToolCallParam
					for _, tc := range tcs {
						if tc.ID == "" {
							valid = false
							break
						}
						toolCallParams = append(toolCallParams, openai.ChatCompletionMessageToolCallParam{
							ID:   tc.ID,
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: tc.ArgsJSON,
							},
						})
					}
					if valid {
						param.OfAssistant.ToolCalls = toolCallParams
						result = append(result, param)
						break
					}
				}
			}
			result = append(result, openai.AssistantMessage(m.Content))
		case "tool":
			result = append(result, openai.ToolMessage(m.Content, m.ToolCallID))
		}
	}

	return result
}

// MaxToolCallsPerReply caps the tool calls one reply may run. A model that keeps calling tools
// would otherwise loop, and pay for every request, until it is stopped by hand.
const MaxToolCallsPerReply = 50

// toolLimitResult is what a tool call past MaxToolCallsPerReply gets instead of running.
var toolLimitResult = fmt.Sprintf("Not run: this reply already used the limit of %d tool calls. Do not call more tools; answer now with what you have, and say what is still missing.", MaxToolCallsPerReply)

// stoppedResult is what the tool calls left in a batch get once the reply has been stopped.
const stoppedResult = "Not run: the user stopped the reply."

// StreamingChat runs an OpenAI-compatible streaming chat loop with tool calling.
// This is the shared "engine" used by all providers that speak the OpenAI protocol.
//
// It returns the messages the turn generated. When the stream fails or ctx is cancelled it
// still returns what was generated until then, ending with the partly streamed reply, so the
// caller can keep it.
func StreamingChat(
	ctx context.Context,
	client openai.Client,
	model string,
	registry *tools.Registry,
	messages []shared.Message,
	imageDataURLs []string,
	systemPrompt string,
	onChunk func(string),
	onToolCall func(string, string, string),
	onRequestDone func(shared.RequestUsage),
	onRequestCaptured func(shared.CapturedRequest),
) ([]shared.Message, error) {
	var generated []shared.Message
	// Tools called earlier in the conversation stay loaded, so a follow-up
	// (e.g. "fix the text in that image") needs no new tool_search. Tools a
	// search merely returned are not kept, so their schemas are not sent with
	// every later request.
	unlockedTools := toolsCalledInHistory(messages)
	apiMessages := BuildMessages(withoutToolSearches(messages), imageDataURLs, systemPrompt)
	toolDefs := toolDefinitionsWith(registry, unlockedTools)
	toolCallsRun := 0

	for {
		limitReached := toolCallsRun >= MaxToolCallsPerReply
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(model),
			Messages: apiMessages,
			Tools:    toolDefs,
			// Request token usage in the streamed response. Without this, the
			// provider omits usage and we can't report per-request context size.
			StreamOptions: openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: openai.Bool(true),
			},
		}
		if len(toolDefs) > 0 && requiresReasoningNone(model) {
			params.ReasoningEffort = reasoningEffortNone
		}
		if limitReached && len(toolDefs) > 0 {
			// The tools stay listed, so the request still matches the tool calls in the
			// history, but the model must answer in text now.
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("none")}
		}

		// Capture the exact HTTP request body about to be sent to the provider.
		if onRequestCaptured != nil {
			if body, err := json.Marshal(params); err == nil {
				onRequestCaptured(shared.CapturedRequest{RequestBody: string(body)})
			}
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		var streamed strings.Builder

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			for _, choice := range chunk.Choices {
				if choice.Delta.Content == "" {
					continue
				}
				streamed.WriteString(choice.Delta.Content)
				if onChunk != nil {
					onChunk(choice.Delta.Content)
				}
			}
		}

		if err := stream.Err(); err != nil {
			// The provider refuses function tools together with reasoning on the
			// chat completions API. Remember the model and retry with
			// reasoning_effort="none" so runs keep working with tools.
			if len(toolDefs) > 0 && params.ReasoningEffort == "" && len(acc.Choices) == 0 && isToolReasoningConflict(err) {
				markRequiresReasoningNone(model)
				continue
			}
			// Keep what the person already saw stream in, e.g. when they stopped the reply.
			if partial := streamed.String(); strings.TrimSpace(partial) != "" {
				generated = append(generated, shared.Message{Role: "assistant", Content: partial})
			}
			return generated, fmt.Errorf("streaming error: %w", err)
		}

		// Report usage for this completed HTTP request (only populated on the
		// final chunk for most providers; zero when not returned).
		if onRequestDone != nil {
			onRequestDone(shared.RequestUsage{
				PromptTokens:     int(acc.Usage.PromptTokens),
				CompletionTokens: int(acc.Usage.CompletionTokens),
			})
		}

		if len(acc.Choices) == 0 {
			break
		}

		msg := acc.Choices[0].Message
		if len(msg.ToolCalls) == 0 || limitReached {
			// Final assistant text message. Past the limit, tool calls are dropped even when
			// a provider ignores tool_choice "none", so the loop always ends.
			generated = append(generated, shared.Message{
				Role:    "assistant",
				Content: msg.Content,
			})
			break
		}
		if registry == nil {
			return generated, fmt.Errorf("model requested tool calls but no tool registry is configured")
		}

		// Record the assistant message with tool calls.
		var toolCallRecs []toolCallRecord
		for _, tc := range msg.ToolCalls {
			toolCallRecs = append(toolCallRecs, toolCallRecord{
				Name:     tc.Function.Name,
				ArgsJSON: tc.Function.Arguments,
				ID:       tc.ID,
			})
		}
		toolCallsJSON, _ := json.Marshal(toolCallRecs)
		generated = append(generated, shared.Message{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: string(toolCallsJSON),
		})

		apiMessages = append(apiMessages, msg.ToParam())

		for _, tc := range msg.ToolCalls {
			// Every call gets a result, even one that does not run: the API rejects tool
			// calls without a matching tool message.
			var result string
			switch {
			case ctx.Err() != nil:
				result = stoppedResult
			case toolCallsRun >= MaxToolCallsPerReply:
				result = toolLimitResult
			default:
				toolCallsRun++
				if onToolCall != nil {
					onToolCall(tc.Function.Name, tc.Function.Arguments, tc.ID)
				}
				result = registry.ExecuteContext(ctx, tc.Function.Name, tc.Function.Arguments)
				if tc.Function.Name == tools.ToolSearchToolName {
					handleToolSearchResult(result, unlockedTools)
				}
			}

			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
			generated = append(generated, shared.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		toolDefs = toolDefinitionsWith(registry, unlockedTools)
	}

	return generated, nil
}

// toolDefinitionsWith returns the always-visible tools plus the unlocked
// deferred ones. Non-deferred names are skipped, as they are already listed.
func toolDefinitionsWith(registry *tools.Registry, unlocked map[string]struct{}) []openai.ChatCompletionToolParam {
	defs := ToolDefinitions(registry)
	if registry == nil || len(unlocked) == 0 {
		return defs
	}
	names := make([]string, 0, len(unlocked))
	for name := range unlocked {
		names = append(names, name)
	}
	sort.Strings(names)
	var extra []tools.Definition
	for _, def := range registry.DefinitionsByName(names) {
		if def.DeferLoading {
			extra = append(extra, def)
		}
	}
	return append(defs, toOpenAIToolDefinitions(extra)...)
}

// withoutToolSearches drops earlier turns' tool_search calls and their
// results. The tools they loaded stay loaded (see toolsLoadedInHistory), and
// left in, they teach the model to search again at the start of every turn.
func withoutToolSearches(messages []shared.Message) []shared.Message {
	out := make([]shared.Message, 0, len(messages))
	dropped := map[string]struct{}{}
	for _, m := range messages {
		switch {
		case m.Role == "assistant" && m.ToolCalls != "":
			var tcs []toolCallRecord
			if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err != nil {
				out = append(out, m)
				continue
			}
			var kept []toolCallRecord
			for _, tc := range tcs {
				if tc.Name == tools.ToolSearchToolName {
					dropped[tc.ID] = struct{}{}
				} else {
					kept = append(kept, tc)
				}
			}
			switch {
			case len(kept) == len(tcs):
				out = append(out, m)
			case len(kept) > 0:
				data, _ := json.Marshal(kept)
				out = append(out, shared.Message{Role: m.Role, Content: m.Content, ToolCalls: string(data)})
			case strings.TrimSpace(m.Content) != "":
				out = append(out, shared.Message{Role: m.Role, Content: m.Content})
			}
		case m.Role == "tool":
			if _, ok := dropped[m.ToolCallID]; !ok {
				out = append(out, m)
			}
		default:
			out = append(out, m)
		}
	}
	return out
}

// toolsCalledInHistory collects the tools earlier turns called, apart from
// tool_search itself.
func toolsCalledInHistory(messages []shared.Message) map[string]struct{} {
	called := map[string]struct{}{}
	for _, m := range messages {
		if m.Role != "assistant" || m.ToolCalls == "" {
			continue
		}
		var tcs []toolCallRecord
		if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err != nil {
			continue
		}
		for _, tc := range tcs {
			if tc.Name != "" && tc.Name != tools.ToolSearchToolName {
				called[tc.Name] = struct{}{}
			}
		}
	}
	return called
}

type toolSearchResultJSON struct {
	ToolReference []struct {
		ToolName string `json:"tool_name"`
	} `json:"tool_references"`
}

func handleToolSearchResult(result string, unlocked map[string]struct{}) {
	if unlocked == nil {
		return
	}

	var parsed toolSearchResultJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return
	}

	for _, ref := range parsed.ToolReference {
		if ref.ToolName != "" {
			unlocked[ref.ToolName] = struct{}{}
		}
	}
}

// ExtractCompletion runs a non-streaming completion for memory extraction.
func ExtractCompletion(
	ctx context.Context,
	client openai.Client,
	model string,
	systemPrompt string,
	userContent string,
) (string, error) {
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userContent),
		},
	}
	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("extraction completion error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("extraction returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}
