package azure_apikey

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"pedro/providers/openaiutil"
	"pedro/shared"
	"pedro/tools"
)

const DefaultAPIVersion = "2024-12-01-preview"

type Config struct {
	Endpoint   string
	Deployment string
	APIVersion string
	APIKey     string
}

func (c Config) Type() string { return "azure_apikey" }

func (c Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("azure endpoint is required")
	}
	if c.Deployment == "" {
		return errors.New("azure deployment is required")
	}
	if c.APIKey == "" {
		return errors.New("azure api key is required")
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
	authenticated      bool
}

func (p *Provider) Name() string                    { return "azure_apikey" }
func (p *Provider) IsAuthenticated() bool            { return p.authenticated }
func (p *Provider) SetAuthenticated(a bool)          { p.authenticated = a }
func (p *Provider) SetBaseSystemPrompt(s string)     { p.baseSystemPrompt = s }
func (p *Provider) SetCustomSystemPrompt(s string)   { p.customSystemPrompt = s }
func (p *Provider) SetPersonaPrompt(s string)        { p.personaPrompt = s }

func ParseConfig(settings map[string]string) (shared.Config, error) {
	cfg := Config{
		Endpoint:   settings["azure_endpoint"],
		Deployment: settings["azure_deployment"],
		APIVersion: settings["azure_api_version"],
		APIKey:     settings["azure_api_key"],
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	return cfg, nil
}

// Build creates a fully-configured Azure API-key provider.
func Build(cfg shared.Config, _ shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
	c, ok := cfg.(Config)
	if !ok {
		return nil, fmt.Errorf("azure_apikey: expected Config, got %T", cfg)
	}

	apiVersion := c.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	return &Provider{
		client: openai.NewClient(
			azure.WithEndpoint(c.Endpoint, apiVersion),
			azure.WithAPIKey(c.APIKey),
		),
		config:        c,
		registry:      registry,
		authenticated: c.APIKey != "",
	}, nil
}

func (p *Provider) SignIn(_ context.Context) error {
	if p.config.APIKey == "" {
		return errors.New("API key not configured")
	}
	p.authenticated = true
	return nil
}

func (p *Provider) SignOut() error {
	p.authenticated = false
	return nil
}

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	if !p.authenticated {
		return fmt.Errorf("not authenticated – API key may be missing")
	}

	base := p.baseSystemPrompt
	if base == "" {
		base = shared.DefaultSystemPrompt
	}
	prompt := openaiutil.FullSystemPrompt(base, p.personaPrompt, p.customSystemPrompt)
	return openaiutil.StreamingChat(ctx, p.client, p.config.Deployment, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall)
}
