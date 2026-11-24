package azure_apikey

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"pedro/media"
	"pedro/providers/openaiutil"
	"pedro/shared"
	"pedro/tools"
)

const DefaultAPIVersion = "2024-12-01-preview"

// MediaAPIVersion is used for image generation and transcription; gpt-image
// deployments need a newer API version than chat.
const MediaAPIVersion = "2025-04-01-preview"

// Default media deployments; an explicitly empty setting disables the feature.
const (
	DefaultImageDeployment         = "gpt-image-2.5-sunburst"
	DefaultTranscriptionDeployment = "gpt-transcribe"
)

type Config struct {
	Endpoint   string
	Deployment string
	APIVersion string
	APIKey     string
	// ImageDeployment and TranscriptionDeployment live on the same resource
	// and share the API key. Empty disables the capability.
	ImageDeployment         string
	TranscriptionDeployment string
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
	mediaClient        openai.Client
	config             Config
	registry           *tools.Registry
	baseSystemPrompt   string
	customSystemPrompt string
	memoryContext      string
	authenticated      bool
}

func (p *Provider) Name() string                   { return "azure_apikey" }
func (p *Provider) IsAuthenticated() bool          { return p.authenticated }
func (p *Provider) SetAuthenticated(a bool)        { p.authenticated = a }
func (p *Provider) SetBaseSystemPrompt(s string)   { p.baseSystemPrompt = s }
func (p *Provider) SetCustomSystemPrompt(s string) { p.customSystemPrompt = s }
func (p *Provider) SetMemoryContext(s string)      { p.memoryContext = s }
func (p *Provider) ExtractionClient() any          { return p.client }
func (p *Provider) ModelName() string              { return p.config.Deployment }

func ParseConfig(settings map[string]string) (shared.Config, error) {
	cfg := Config{
		Endpoint:                settings["azure_endpoint"],
		Deployment:              settings["azure_deployment"],
		APIVersion:              settings["azure_api_version"],
		APIKey:                  settings["azure_api_key"],
		ImageDeployment:         settingOrDefault(settings, "azure_image_deployment", DefaultImageDeployment),
		TranscriptionDeployment: settingOrDefault(settings, "azure_transcription_deployment", DefaultTranscriptionDeployment),
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	return cfg, nil
}

// settingOrDefault returns the trimmed setting, or def when the key was never saved.
func settingOrDefault(settings map[string]string, key, def string) string {
	v, ok := settings[key]
	if !ok {
		return def
	}
	return strings.TrimSpace(v)
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
		mediaClient: openai.NewClient(
			azure.WithEndpoint(c.Endpoint, MediaAPIVersion),
			azure.WithAPIKey(c.APIKey),
			withImageEditDeploymentRoute(c.ImageDeployment),
		),
		config:        c,
		registry:      registry,
		authenticated: c.APIKey != "",
	}, nil
}

// withImageEditDeploymentRoute sends image edits to the image deployment.
// The SDK's Azure middleware does this for generations but not for edits,
// which would otherwise hit /openai/images/edits and fail with 404.
func withImageEditDeploymentRoute(deployment string) option.RequestOption {
	const editsPath = "/openai/images/edits"
	return option.WithMiddleware(func(r *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		if deployment != "" && strings.HasSuffix(r.URL.Path, editsPath) {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, editsPath) + "/openai/deployments/" + url.PathEscape(deployment) + "/images/edits"
		}
		return next(r)
	})
}

// MediaServices exposes image generation and transcription on the same resource.
func (p *Provider) MediaServices() media.Services {
	var s media.Services
	if p.config.ImageDeployment != "" {
		s.Images = media.NewOpenAIImageGenerator(p.mediaClient, p.config.ImageDeployment)
	}
	if p.config.TranscriptionDeployment != "" {
		s.Transcriber = media.NewOpenAITranscriber(p.mediaClient, p.config.TranscriptionDeployment)
	}
	return s
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

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON, id string), onRequestDone func(shared.RequestUsage), onRequestCaptured func(shared.CapturedRequest)) ([]shared.Message, error) {
	if !p.authenticated {
		return nil, fmt.Errorf("not authenticated – API key may be missing")
	}

	base := p.baseSystemPrompt
	if base == "" {
		base = shared.DefaultSystemPrompt
	}
	prompt := openaiutil.FullSystemPrompt(base, p.customSystemPrompt, p.memoryContext)
	return openaiutil.StreamingChat(ctx, p.client, p.config.Deployment, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall, onRequestDone, onRequestCaptured)
}
