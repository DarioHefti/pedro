package providers

import (
	"errors"
	"fmt"

	"pedro/shared"
	"pedro/tools"
)

type ProviderType string

const (
	ProviderAzure     ProviderType = "azure"
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
)

var ErrInvalidProvider = errors.New("invalid provider type")

type BuilderFunc func(registry *tools.Registry) (shared.LLMClient, error)

type Factory struct {
	builders map[ProviderType]BuilderFunc
	parsers  map[ProviderType]func(map[string]string) (shared.Config, error)
}

func NewFactory() *Factory {
	return &Factory{
		builders: make(map[ProviderType]BuilderFunc),
		parsers:  make(map[ProviderType]func(map[string]string) (shared.Config, error)),
	}
}

func (f *Factory) Register(provider ProviderType, builder BuilderFunc, parser func(map[string]string) (shared.Config, error)) {
	f.builders[provider] = builder
	f.parsers[provider] = parser
}

func (f *Factory) Create(cfg shared.Config, registry *tools.Registry) (shared.LLMClient, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	builder, ok := f.builders[ProviderType(cfg.Type())]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProvider, cfg.Type())
	}

	return builder(registry)
}

func (f *Factory) ParseSettings(settings map[string]string) (shared.Config, error) {
	providerType := ProviderType(settings["provider_type"])
	parser, ok := f.parsers[providerType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProvider, providerType)
	}
	return parser(settings)
}

func (f *Factory) RegisteredProviders() []ProviderType {
	types := make([]ProviderType, 0, len(f.builders))
	for t := range f.builders {
		types = append(types, t)
	}
	return types
}

var defaultFactory *Factory

func DefaultFactory() *Factory {
	if defaultFactory == nil {
		defaultFactory = NewFactory()
	}
	return defaultFactory
}
