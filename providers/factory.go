package providers

import (
	"errors"
	"fmt"

	"pedro/shared"
	"pedro/tools"
)

type ProviderType string

const (
	ProviderAzure       ProviderType = "azure"
	ProviderAzureAPIKey ProviderType = "azure_apikey"
	ProviderOpenAI      ProviderType = "openai"
)

var ErrInvalidProvider = errors.New("invalid provider type")

// BuilderFunc creates a fully-configured LLMClient from a validated config,
// a settings store (for persisting auth tokens etc.), and a tool registry.
type BuilderFunc func(cfg shared.Config, store shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error)

type ProviderDescriptor struct {
	ID   ProviderType
	Name string
}

type Factory struct {
	builders    map[ProviderType]BuilderFunc
	parsers     map[ProviderType]func(map[string]string) (shared.Config, error)
	descriptors map[ProviderType]ProviderDescriptor
	order       []ProviderType
}

func NewFactory() *Factory {
	return &Factory{
		builders:    make(map[ProviderType]BuilderFunc),
		parsers:     make(map[ProviderType]func(map[string]string) (shared.Config, error)),
		descriptors: make(map[ProviderType]ProviderDescriptor),
		order:       []ProviderType{},
	}
}

func (f *Factory) Register(provider ProviderType, descriptor ProviderDescriptor, builder BuilderFunc, parser func(map[string]string) (shared.Config, error)) {
	if _, exists := f.builders[provider]; !exists {
		f.order = append(f.order, provider)
	}
	if descriptor.ID == "" {
		descriptor.ID = provider
	}
	if descriptor.Name == "" {
		descriptor.Name = string(provider)
	}

	f.builders[provider] = builder
	f.parsers[provider] = parser
	f.descriptors[provider] = descriptor
}

// Create validates the config, then delegates to the registered builder.
// The returned LLMClient is fully configured and ready to use.
func (f *Factory) Create(cfg shared.Config, store shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
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

	return builder(cfg, store, registry)
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
	types := make([]ProviderType, 0, len(f.order))
	for _, t := range f.order {
		types = append(types, t)
	}
	return types
}

func (f *Factory) RegisteredProviderDescriptors() []ProviderDescriptor {
	descriptors := make([]ProviderDescriptor, 0, len(f.order))
	for _, t := range f.order {
		if descriptor, ok := f.descriptors[t]; ok {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors
}
