package compat

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

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

func (c Config) Type() string { return "compat" }

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}
	if c.Model == "" {
		return errors.New("model name is required")
	}
	return nil
}

type Provider struct {
	client             openai.Client
	config             Config
	registry           *tools.Registry
	baseSystemPrompt   string
	customSystemPrompt string
	personaPrompt      string
	memoryContext      string
	authenticated      bool
}

func (p *Provider) Name() string                   { return "compat" }
func (p *Provider) IsAuthenticated() bool          { return p.authenticated }
func (p *Provider) SetAuthenticated(a bool)        { p.authenticated = a }
func (p *Provider) SetBaseSystemPrompt(s string)   { p.baseSystemPrompt = s }
func (p *Provider) SetCustomSystemPrompt(s string) { p.customSystemPrompt = s }
func (p *Provider) SetPersonaPrompt(s string)      { p.personaPrompt = s }
func (p *Provider) SetMemoryContext(s string)      { p.memoryContext = s }
func (p *Provider) ExtractionClient() any          { return p.client }
func (p *Provider) ModelName() string              { return p.config.Model }

func ParseConfig(settings map[string]string) (shared.Config, error) {
	return Config{
		BaseURL: settings["compat_base_url"],
		APIKey:  settings["compat_api_key"],
		Model:   settings["compat_model"],
	}, nil
}

// Build creates a fully-configured OpenAI-compatible (local) provider.
// Works with any server speaking the OpenAI chat protocol, e.g. LM Studio,
// llama.cpp, Ollama, Oobabooga, etc.
func Build(cfg shared.Config, _ shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
	c, ok := cfg.(Config)
	if !ok {
		return nil, fmt.Errorf("compat: expected Config, got %T", cfg)
	}

	opts := []option.RequestOption{option.WithBaseURL(c.BaseURL)}
	// Local servers like LM Studio accept an arbitrary (or empty) key; the
	// openai-go client requires the header to be set, so default to a dummy.
	key := c.APIKey
	if key == "" {
		key = "not-needed"
	}
	opts = append(opts, option.WithAPIKey(key))

	return &Provider{
		client:        openai.NewClient(opts...),
		config:        c,
		registry:      registry,
		authenticated: true,
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

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON, id string), onRequestDone func(shared.RequestUsage), onRequestCaptured func(shared.CapturedRequest)) ([]shared.Message, error) {
	if !p.authenticated {
		return nil, fmt.Errorf("not authenticated")
	}

	base := p.baseSystemPrompt
	if base == "" {
		base = shared.DefaultSystemPrompt
	}
	prompt := openaiutil.FullSystemPrompt(base, p.personaPrompt, p.customSystemPrompt, p.memoryContext)
	return openaiutil.StreamingChat(ctx, p.client, p.config.Model, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall, onRequestDone, onRequestCaptured)
}
