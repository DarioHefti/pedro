package openai

import "testing"

func TestOpenAIConfigValidate(t *testing.T) {
	if err := (OpenAIConfig{}).Validate(); err == nil {
		t.Fatal("empty api key should fail")
	}
	if err := (OpenAIConfig{APIKey: "sk-test"}).Validate(); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"openai_api_key": "k",
		"openai_model":   "gpt-4o-mini",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := cfg.(OpenAIConfig)
	if !ok {
		t.Fatalf("expected OpenAIConfig, got %T", cfg)
	}
	if c.APIKey != "k" || c.Model != "gpt-4o-mini" {
		t.Fatalf("parsed: %+v", c)
	}
	if c.Type() != "openai" {
		t.Fatalf("Type: %q", c.Type())
	}
}

type wrongOpenAIConfig struct{}

func (wrongOpenAIConfig) Type() string    { return "other" }
func (wrongOpenAIConfig) Validate() error { return nil }

func TestBuildRejectsWrongConfigType(t *testing.T) {
	_, err := Build(wrongOpenAIConfig{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}
