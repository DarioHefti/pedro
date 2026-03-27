package main

import "testing"

func TestConnectionSettingsFingerprint_stable(t *testing.T) {
	m := map[string]string{
		"provider_type":          "azure",
		"azure_endpoint":         "https://x.openai.azure.com",
		"azure_deployment":       "d",
		"azure_tenant_id":        "  t  ",
		"system_prompt":          "",
		"custom_system_prompt":   "",
		"welcome_message":        "Welcome to Pedro",
		"ignored_extra":          "x",
	}
	a := connectionSettingsFingerprint(m)
	b := connectionSettingsFingerprint(m)
	if a != b {
		t.Fatalf("fingerprint not stable: %q vs %q", a, b)
	}
	if a == "" {
		t.Fatal("empty fingerprint")
	}
}
