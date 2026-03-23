package main

import (
	"context"
	"io"
	"strings"

	"pedro/tools"

	"github.com/sashabaranov/go-openai"
)

type AzureClient struct {
	client     *openai.Client
	deployment string
	registry   *tools.Registry
}

func NewAzureClient(endpoint, apiKey, deployment string, registry *tools.Registry) (*AzureClient, error) {
	config := openai.DefaultAzureConfig(apiKey, endpoint)
	config.APIVersion = "2024-12-01-preview"
	config.AzureModelMapperFunc = func(model string) string {
		return deployment
	}
	client := openai.NewClientWithConfig(config)

	return &AzureClient{
		client:     client,
		deployment: deployment,
		registry:   registry,
	}, nil
}

// toolDefinitions converts the registry's provider-agnostic definitions to
// OpenAI tool objects.
func (a *AzureClient) toolDefinitions() []openai.Tool {
	if a.registry == nil {
		return nil
	}
	var result []openai.Tool
	for _, def := range a.registry.Definitions() {
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return result
}

func (a *AzureClient) Chat(ctx context.Context, messages []Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	// Always prepend the system prompt, then append stored messages
	openaiMsgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: SystemPrompt},
	}
	for i, m := range messages {
		if m.Role != "user" && m.Role != "assistant" && m.Role != "system" {
			continue
		}
		// Attach images to the last user message only
		isLastUserMsg := m.Role == "user" && i == len(messages)-1 && len(imageDataURLs) > 0
		if isLastUserMsg {
			parts := []openai.ChatMessagePart{
				{Type: openai.ChatMessagePartTypeText, Text: m.Content},
			}
			for _, imgURL := range imageDataURLs {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    imgURL,
						Detail: openai.ImageURLDetailAuto,
					},
				})
			}
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
				Role:         m.Role,
				MultiContent: parts,
			})
		} else {
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	toolDefs := a.toolDefinitions()

	// Agentic loop: keep calling LLM until it stops requesting tool calls
	for {
		req := openai.ChatCompletionRequest{
			Model:    a.deployment,
			Messages: openaiMsgs,
			Stream:   true,
			Tools:    toolDefs,
		}

		stream, err := a.client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return err
		}

		var contentBuilder strings.Builder
		toolCallMap := make(map[int]*openai.ToolCall)

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				stream.Close()
				return err
			}
			if len(resp.Choices) == 0 {
				continue
			}

			delta := resp.Choices[0].Delta

			if delta.Content != "" {
				contentBuilder.WriteString(delta.Content)
				onChunk(delta.Content)
			}

			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				if _, ok := toolCallMap[idx]; !ok {
					toolCallMap[idx] = &openai.ToolCall{Index: tc.Index}
				}
				acc := toolCallMap[idx]
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Type != "" {
					acc.Type = tc.Type
				}
				acc.Function.Name += tc.Function.Name
				acc.Function.Arguments += tc.Function.Arguments
			}
		}
		stream.Close()

		if len(toolCallMap) == 0 {
			return nil
		}

		toolCalls := make([]openai.ToolCall, len(toolCallMap))
		for idx, tc := range toolCallMap {
			if idx < len(toolCalls) {
				toolCalls[idx] = *tc
			}
		}

		assistantMsg := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			ToolCalls: toolCalls,
		}
		if contentBuilder.Len() > 0 {
			assistantMsg.Content = contentBuilder.String()
		}
		openaiMsgs = append(openaiMsgs, assistantMsg)

		for _, tc := range toolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}
			result := a.registry.Execute(tc.Function.Name, tc.Function.Arguments)
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
}
