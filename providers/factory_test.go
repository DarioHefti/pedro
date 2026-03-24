package providers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pedro/shared"
	"pedro/tools"
)

type stubConfig struct {
	providerType string
	validateErr  error
}

func (c stubConfig) Type() string { return c.providerType }

func (c stubConfig) Validate() error { return c.validateErr }

type stubClient struct{}

func (s *stubClient) Chat(context.Context, []shared.Message, []string, func(string), func(string, string)) error {
	return nil
}
func (s *stubClient) SetCustomSystemPrompt(string) {}
func (s *stubClient) SignIn(context.Context) error { return nil }
func (s *stubClient) SignOut() error               { return nil }
func (s *stubClient) IsAuthenticated() bool        { return true }
func (s *stubClient) SetAuthenticated(bool)        {}
func (s *stubClient) Name() string                 { return "stub" }

func dummyBuilder(shared.Config, shared.SettingsStore, *tools.Registry) (shared.LLMClient, error) {
	return &stubClient{}, nil
}

func dummyParser(map[string]string) (shared.Config, error) {
	return stubConfig{providerType: "dummy"}, nil
}

func TestRegisteredProviderDescriptorsInRegistrationOrder(t *testing.T) {
	f := NewFactory()
	f.Register(ProviderType("p1"), ProviderDescriptor{ID: "p1", Name: "Provider One"}, dummyBuilder, dummyParser)
	f.Register(ProviderType("p2"), ProviderDescriptor{ID: "p2", Name: "Provider Two"}, dummyBuilder, dummyParser)

	gotProviders := f.RegisteredProviders()
	wantProviders := []ProviderType{"p1", "p2"}
	if !reflect.DeepEqual(gotProviders, wantProviders) {
		t.Fatalf("registered providers order mismatch: got %v want %v", gotProviders, wantProviders)
	}

	gotDescriptors := f.RegisteredProviderDescriptors()
	wantDescriptors := []ProviderDescriptor{
		{ID: "p1", Name: "Provider One"},
		{ID: "p2", Name: "Provider Two"},
	}
	if !reflect.DeepEqual(gotDescriptors, wantDescriptors) {
		t.Fatalf("registered descriptors mismatch: got %v want %v", gotDescriptors, wantDescriptors)
	}
}

func TestRegisterUpdateDoesNotDuplicateOrder(t *testing.T) {
	f := NewFactory()
	p := ProviderType("p1")
	f.Register(p, ProviderDescriptor{ID: p, Name: "Original"}, dummyBuilder, dummyParser)
	f.Register(p, ProviderDescriptor{ID: p, Name: "Updated"}, dummyBuilder, dummyParser)

	gotProviders := f.RegisteredProviders()
	if len(gotProviders) != 1 || gotProviders[0] != p {
		t.Fatalf("expected single provider %q, got %v", p, gotProviders)
	}

	gotDescriptors := f.RegisteredProviderDescriptors()
	if len(gotDescriptors) != 1 || gotDescriptors[0].Name != "Updated" {
		t.Fatalf("expected updated descriptor name, got %v", gotDescriptors)
	}
}

func TestCreateValidatesAndUsesBuilder(t *testing.T) {
	f := NewFactory()
	providerType := ProviderType("valid")
	called := false

	f.Register(providerType, ProviderDescriptor{ID: providerType, Name: "Valid"}, func(cfg shared.Config, store shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
		called = true
		return &stubClient{}, nil
	}, dummyParser)

	cfg := stubConfig{providerType: string(providerType)}
	client, err := f.Create(cfg, nil, nil)
	if err != nil {
		t.Fatalf("Create() returned unexpected error: %v", err)
	}
	if client == nil {
		t.Fatalf("Create() returned nil client")
	}
	if !called {
		t.Fatalf("expected builder to be called")
	}
}

func TestCreateErrors(t *testing.T) {
	f := NewFactory()

	if _, err := f.Create(nil, nil, nil); err == nil {
		t.Fatalf("expected error for nil config")
	}

	cfgWithValidationErr := stubConfig{providerType: "missing", validateErr: errors.New("bad config")}
	if _, err := f.Create(cfgWithValidationErr, nil, nil); err == nil {
		t.Fatalf("expected validation error")
	}

	cfgUnknownProvider := stubConfig{providerType: "missing"}
	if _, err := f.Create(cfgUnknownProvider, nil, nil); err == nil {
		t.Fatalf("expected invalid provider error")
	}
}

func TestParseSettings(t *testing.T) {
	f := NewFactory()
	providerType := ProviderType("dummy")
	f.Register(providerType, ProviderDescriptor{ID: providerType, Name: "Dummy"}, dummyBuilder, func(settings map[string]string) (shared.Config, error) {
		if settings["custom"] != "ok" {
			return nil, errors.New("unexpected settings")
		}
		return stubConfig{providerType: string(providerType)}, nil
	})

	cfg, err := f.ParseSettings(map[string]string{
		"provider_type": "dummy",
		"custom":        "ok",
	})
	if err != nil {
		t.Fatalf("ParseSettings() returned unexpected error: %v", err)
	}
	if cfg.Type() != "dummy" {
		t.Fatalf("expected parsed config type dummy, got %q", cfg.Type())
	}

	if _, err := f.ParseSettings(map[string]string{"provider_type": "unknown"}); err == nil {
		t.Fatalf("expected invalid provider error for unknown parser")
	}
}

func TestRegisterProvidersStableOrderAndIDs(t *testing.T) {
	f := NewFactory()
	RegisterProviders(f)

	got := f.RegisteredProviders()
	want := []ProviderType{ProviderAzure, ProviderAzureAPIKey, ProviderOpenAI}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider order: got %v want %v", got, want)
	}

	desc := f.RegisteredProviderDescriptors()
	if len(desc) != len(want) {
		t.Fatalf("descriptor count: got %d want %d", len(desc), len(want))
	}
	for i, d := range desc {
		if d.ID != want[i] {
			t.Fatalf("descriptor[%d].ID: got %q want %q", i, d.ID, want[i])
		}
		if d.Name == "" {
			t.Fatalf("descriptor[%d] empty name", i)
		}
	}
}
