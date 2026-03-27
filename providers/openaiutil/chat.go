package openaiutil

import (
	"context"
	"fmt"

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
	var result []openai.ChatCompletionToolParam
	for _, def := range registry.Definitions() {
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
			result := registry.Execute(tc.Function.Name, tc.Function.Arguments)
			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
		}
	}

	return nil
}
