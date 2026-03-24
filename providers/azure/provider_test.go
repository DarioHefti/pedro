package azure

import (
	"errors"
	"fmt"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("empty config should fail validation")
	}
	if err := (Config{Endpoint: "https://x.cognitiveservices.azure.com"}).Validate(); err == nil {
		t.Fatal("missing deployment should fail")
	}
	if err := (Config{
		Endpoint:   "https://x.openai.azure.com",
		Deployment: "gpt-4",
	}).Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestParseConfigDefaultsAPIVersion(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"azure_endpoint":   "https://res.openai.azure.com",
		"azure_deployment": "dep",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := cfg.(Config)
	if !ok {
		t.Fatalf("expected Config, got %T", cfg)
	}
	if c.APIVersion != DefaultAPIVersion {
		t.Fatalf("default API version: got %q want %q", c.APIVersion, DefaultAPIVersion)
	}
	if c.Type() != "azure" {
		t.Fatalf("Type: got %q", c.Type())
	}
}

func TestParseConfigTrimsTenantID(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"azure_endpoint":   "https://res.openai.azure.com",
		"azure_deployment": "dep",
		"azure_tenant_id":  "  tenant-guid  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.(Config)
	if c.TenantID != "tenant-guid" {
		t.Fatalf("tenant trim: got %q", c.TenantID)
	}
}

func TestRequiresInteractiveAuth(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("InteractiveBrowserCredential can't acquire a token without user interaction"), true},
		{errors.New("Call Authenticate to authenticate a user interactively"), true},
		{fmt.Errorf("wrap: %w", errors.New("InteractiveBrowserCredential can't acquire a token without user interaction")), true},
		{errors.New("unrelated error"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := requiresInteractiveAuth(tc.err); got != tc.want {
			t.Fatalf("requiresInteractiveAuth(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

type wrongAzureConfig struct{}

func (wrongAzureConfig) Type() string    { return "other" }
func (wrongAzureConfig) Validate() error { return nil }

func TestBuildRejectsWrongConfigType(t *testing.T) {
	_, err := Build(wrongAzureConfig{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}
