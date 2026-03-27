package openaiutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"pedro/shared"
	"pedro/tools"
)

// FullSystemPrompt builds the final system prompt from the base instructions,
// an optional persona, and optional custom instructions.
func FullSystemPrompt(base, persona, custom string) string {
	out := base
	if persona != "" {
		out += "\n\n## Persona\nYou MUST adopt the following persona for ALL your responses. " +
			"This overrides your default tone, style, and personality:\n" + persona
	}
	if custom != "" {
		out += "\n\n## Additional Instructions\n" + custom
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
			result = append(result, openai.AssistantMessage(m.Content))
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
	onToolCall func(string, string),
) error {
	apiMessages := BuildMessages(messages, imageDataURLs, systemPrompt)
	toolDefs := ToolDefinitions(registry)
	unlockedTools := map[string]struct{}{}

	for {
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(model),
			Messages: apiMessages,
			Tools:    toolDefs,
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
			return fmt.Errorf("streaming error: %w", err)
		}

		if len(acc.Choices) == 0 {
			break
		}

		msg := acc.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			break
		}
		if registry == nil {
			return fmt.Errorf("model requested tool calls but no tool registry is configured")
		}

		apiMessages = append(apiMessages, msg.ToParam())

		for _, tc := range msg.ToolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}
			if tc.Function.Name == "tool_discovery" {
				maybeUnlockDirectTool(tc.Function.Arguments, registry, unlockedTools)
			}
			result := registry.Execute(tc.Function.Name, tc.Function.Arguments)
			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
		}

		toolDefs = append([]openai.ChatCompletionToolParam{}, ToolDefinitions(registry)...)
		if len(unlockedTools) > 0 {
			names := make([]string, 0, len(unlockedTools))
			for name := range unlockedTools {
				names = append(names, name)
			}
			sort.Strings(names)
			toolDefs = append(toolDefs, toOpenAIToolDefinitions(registry.DefinitionsByName(names))...)
		}
	}

	return nil
}

func maybeUnlockDirectTool(argsJSON string, registry *tools.Registry, unlocked map[string]struct{}) {
	if registry == nil || unlocked == nil {
		return
	}
	var args struct {
		Action   string `json:"action"`
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "list" {
		for _, def := range registry.AllDefinitions() {
			if def.Name == "tool_discovery" {
				continue
			}
			unlocked[def.Name] = struct{}{}
		}
		return
	}
	if action != "describe" && action != "execute" {
		return
	}
	name := strings.TrimSpace(args.ToolName)
	if name == "" || name == "tool_discovery" {
		return
	}
	if _, ok := registry.DefinitionByName(name); !ok {
		return
	}
	unlocked[name] = struct{}{}
}
