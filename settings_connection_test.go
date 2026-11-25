package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestConnectionSettingsFingerprint_stable(t *testing.T) {
	m := map[string]string{
		"provider_type":        "azure",
		"azure_endpoint":       "https://x.openai.azure.com",
		"azure_deployment":     "d",
		"azure_tenant_id":      "  t  ",
		"system_prompt":        "",
		"custom_system_prompt": "",
		"welcome_message":      "Welcome to Pedro",
		"ignored_extra":        "x",
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

// fingerprintCase is shared with the frontend (src/settings/settingsModel.test.ts): both sides
// must derive the same fingerprint from saved settings, or a passed connection test is forgotten.
type fingerprintCase struct {
	Name        string            `json:"name"`
	Settings    map[string]string `json:"settings"`
	Fingerprint string            `json:"fingerprint"`
}

func loadFingerprintCases(t *testing.T) []fingerprintCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/connection_fingerprints.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var cases []fingerprintCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("fixture has no cases")
	}
	return cases
}

func TestConnectionSettingsFingerprint_matchesFrontendContract(t *testing.T) {
	for _, tc := range loadFingerprintCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			if got := connectionSettingsFingerprint(tc.Settings); got != tc.Fingerprint {
				t.Fatalf("fingerprint mismatch\n got: %s\nwant: %s", got, tc.Fingerprint)
			}
		})
	}
}

func TestSettingsKeyInvalidatesConnectionTest(t *testing.T) {
	for _, key := range []string{"provider_type", "compat_model", "welcome_message", "design_ui_font_size_px"} {
		if !settingsKeyInvalidatesConnectionTest(key) {
			t.Errorf("%s should invalidate the stored connection test", key)
		}
	}
	for _, key := range []string{"authenticated", "connection_test", "global_request_count", "lifetime_tokens"} {
		if settingsKeyInvalidatesConnectionTest(key) {
			t.Errorf("%s should not invalidate the stored connection test", key)
		}
	}
}

func TestConnectionTestFailureMessageForStore(t *testing.T) {
	if got := connectionTestFailureMessageForStore("  Error: 401 Unauthorized "); got != "401 Unauthorized" {
		t.Fatalf("got %q", got)
	}
}
