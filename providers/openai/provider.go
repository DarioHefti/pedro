package openai

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"pedro/providers/openaiutil"
	"pedro/shared"
	"pedro/tools"
)

type OpenAIConfig struct {
	APIKey string
	Model  string
}

func (c OpenAIConfig) Type() string { return "openai" }

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

func (p *Provider) Name() string            { return "openai" }
func (p *Provider) IsAuthenticated() bool    { return p.authenticated }
func (p *Provider) SetAuthenticated(a bool)  { p.authenticated = a }
func (p *Provider) SetCustomSystemPrompt(s string) { p.customSystemPrompt = s }

func ParseConfig(settings map[string]string) (shared.Config, error) {
	return OpenAIConfig{
		APIKey: settings["openai_api_key"],
		Model:  settings["openai_model"],
	}, nil
}

// Build creates a fully-configured OpenAI provider.
func Build(cfg shared.Config, _ shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
	c, ok := cfg.(OpenAIConfig)
	if !ok {
		return nil, fmt.Errorf("openai: expected OpenAIConfig, got %T", cfg)
	}

	return &Provider{
		client:        openai.NewClient(option.WithAPIKey(c.APIKey)),
		config:        c,
		registry:      registry,
		authenticated: c.APIKey != "",
	}, nil
}

func (p *Provider) SignIn(_ context.Context) error {
	p.authenticated = true
	return nil
}

func (p *Provider) SignOut() error {
	p.authenticated = false
	return nil
}

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	if !p.authenticated {
		return fmt.Errorf("not authenticated")
	}

	model := p.config.Model
	if model == "" {
		model = "gpt-4o"
	}

	prompt := openaiutil.FullSystemPrompt(shared.SystemPrompt, p.customSystemPrompt)
	return openaiutil.StreamingChat(ctx, p.client, model, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall)
}
