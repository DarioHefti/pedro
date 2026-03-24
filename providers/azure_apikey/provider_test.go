package azure_apikey

import "testing"

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("empty config should fail")
	}
	if err := (Config{
		Endpoint:   "https://x.openai.azure.com",
		Deployment: "d",
		APIKey:     "k",
	}).Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestParseConfigDefaultAPIVersion(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"azure_endpoint":   "https://res.openai.azure.com",
		"azure_deployment": "dep",
		"azure_api_key":    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := cfg.(Config)
	if !ok {
		t.Fatalf("expected Config, got %T", cfg)
	}
	if c.APIVersion != DefaultAPIVersion {
		t.Fatalf("API version: got %q", c.APIVersion)
	}
	if c.Type() != "azure_apikey" {
		t.Fatalf("Type: got %q", c.Type())
	}
}

type wrongAPIKeyConfig struct{}

func (wrongAPIKeyConfig) Type() string    { return "other" }
func (wrongAPIKeyConfig) Validate() error { return nil }

func TestBuildRejectsWrongConfigType(t *testing.T) {
	_, err := Build(wrongAPIKeyConfig{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}
