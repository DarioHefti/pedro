package openaiutil

import (
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"sort"
	"time"

	"github.com/openai/openai-go"
	"pedro/shared"
	"pedro/tools"
)

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

// FullSystemPrompt builds the final system prompt from the base instructions,
// an optional persona, optional custom instructions, and optional memory context.
func FullSystemPrompt(base, persona, custom, memoryCtx string) string {
	now := time.Now().UTC().Format("2006-01-02 15:04:05 MST")
	out := base + "\n\n## Current Date/Time\nCurrent UTC datetime: " + now
	out += fmt.Sprintf(
		"\n\n## Operating System\nThe user is running Pedro on %s (%s/%s). When suggesting terminal commands, file paths, keyboard shortcuts, or other OS-specific steps, use conventions appropriate for this operating system. Default shell: %s.",
		userFacingOSName(),
		goruntime.GOOS,
		goruntime.GOARCH,
		userFacingShellHint(),
	)
	if persona != "" {
		out += "\n\n## Persona\nYou MUST adopt the following persona for ALL your responses. " +
			"This overrides your default tone, style, and personality:\n" + persona
	}
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

// StreamingChat runs an OpenAI-compatible streaming chat loop with tool calling.
// This is the shared "engine" used by all providers that speak the OpenAI protocol.
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
	apiMessages := BuildMessages(messages, imageDataURLs, systemPrompt)
	toolDefs := ToolDefinitions(registry)
	unlockedTools := map[string]struct{}{}

	for {
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

		// Capture the exact HTTP request body about to be sent to the provider.
		if onRequestCaptured != nil {
			if body, err := json.Marshal(params); err == nil {
				onRequestCaptured(shared.CapturedRequest{RequestBody: string(body)})
			}
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" && onChunk != nil {
					onChunk(choice.Delta.Content)
				}
			}
		}

		if err := stream.Err(); err != nil {
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
		if len(msg.ToolCalls) == 0 {
			// Final assistant text message.
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
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments, tc.ID)
			}
			result := registry.Execute(tc.Function.Name, tc.Function.Arguments)

			if tc.Function.Name == tools.ToolSearchToolName {
				handleToolSearchResult(result, unlockedTools)
			}

			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
			generated = append(generated, shared.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		toolDefs = ToolDefinitions(registry)
		if len(unlockedTools) > 0 {
			names := make([]string, 0, len(unlockedTools))
			for name := range unlockedTools {
				names = append(names, name)
			}
			sort.Strings(names)
			toolDefs = append(toolDefs, toOpenAIToolDefinitions(registry.DefinitionsByName(names))...)
		}
	}

	return generated, nil
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
