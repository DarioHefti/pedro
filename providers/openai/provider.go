package openai

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"pedro/shared"
	"pedro/tools"
)

type OpenAIConfig struct {
	APIKey string
	Model  string
}

func (c OpenAIConfig) Type() string {
	return "openai"
}

func (c OpenAIConfig) Validate() error {
	if c.APIKey == "" {
		return errors.New("openai api key is required")
	}
	return nil
}

type Provider struct {
	client             openai.Client
	config             OpenAIConfig
	registry           *tools.Registry
	customSystemPrompt string
	authenticated      bool
}

type Builder struct{}

func (Builder) Build(registry *tools.Registry) (shared.LLMClient, error) {
	return &Provider{
		registry: registry,
	}, nil
}

func ParseConfig(settings map[string]string) (shared.Config, error) {
	cfg := OpenAIConfig{
		APIKey: settings["openai_api_key"],
		Model:  settings["openai_model"],
	}
	return cfg, nil
}

func (p *Provider) Name() string {
	return "openai"
}

func (p *Provider) SetConfig(cfg OpenAIConfig) {
	p.config = cfg
	if cfg.APIKey != "" {
		p.client = openai.NewClient(option.WithAPIKey(cfg.APIKey))
	}
}

func (p *Provider) SignIn(ctx context.Context) error {
	p.authenticated = true
	return nil
}

func (p *Provider) SignOut() error {
	p.authenticated = false
	return nil
}

func (p *Provider) IsAuthenticated() bool {
	return p.authenticated
}

func (p *Provider) SetCustomSystemPrompt(prompt string) {
	p.customSystemPrompt = prompt
}

func (p *Provider) SetAuthenticated(auth bool) {
	p.authenticated = auth
}

func (p *Provider) getFullSystemPrompt(systemPrompt string) string {
	if p.customSystemPrompt == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\n## Additional Instructions\n" + p.customSystemPrompt
}

func (p *Provider) toolDefinitions() []openai.ChatCompletionToolParam {
	if p.registry == nil {
		return nil
	}
	var result []openai.ChatCompletionToolParam
	for _, def := range p.registry.Definitions() {
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

func (p *Provider) buildInitialMessages(messages []shared.Message, imageDataURLs []string, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	result := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(p.getFullSystemPrompt(systemPrompt)),
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

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	if !p.authenticated {
		return fmt.Errorf("not authenticated")
	}

	model := p.config.Model
	if model == "" {
		model = "gpt-4o"
	}

	apiMessages := p.buildInitialMessages(messages, imageDataURLs, "")
	toolDefs := p.toolDefinitions()

	for {
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(model),
			Messages: apiMessages,
			Tools:    toolDefs,
		}

		stream := p.client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
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

		apiMessages = append(apiMessages, msg.ToParam())

		for _, tc := range msg.ToolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}
			result := p.registry.Execute(tc.Function.Name, tc.Function.Arguments)
			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
		}
	}

	return nil
}
