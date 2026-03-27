package main

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	connectionTestSettingKey = "connection_test"
	defaultWelcomeMessage    = "Welcome to Pedro"
)

type connectionTestPersisted struct {
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	At          int64  `json:"at"`
	Fingerprint string `json:"fingerprint"`
}

// connectionSettingsFingerprint matches frontend fingerprintFromSnapshot / buildFullSettingsRecord.
func connectionSettingsFingerprint(settings map[string]string) string {
	pt := settings["provider_type"]
	if pt == "" {
		pt = "azure"
	}

	wm := settings["welcome_message"]
	if wm == "" {
		wm = defaultWelcomeMessage
	}

	rec := map[string]string{
		"provider_type":               pt,
		"custom_system_prompt":        settings["custom_system_prompt"],
		"welcome_message":             wm,
		"design_light_base_color":     settings["design_light_base_color"],
		"design_dark_base_color":      settings["design_dark_base_color"],
		"design_ui_font_size_px":      settings["design_ui_font_size_px"],
		"design_message_font_size_px": settings["design_message_font_size_px"],
	}

	switch pt {
	case "azure":
		rec["azure_endpoint"] = settings["azure_endpoint"]
		rec["azure_deployment"] = settings["azure_deployment"]
		rec["azure_tenant_id"] = strings.TrimSpace(settings["azure_tenant_id"])
	case "azure_apikey":
		rec["azure_endpoint"] = settings["azure_endpoint"]
		rec["azure_deployment"] = settings["azure_deployment"]
		rec["azure_api_key"] = settings["azure_api_key"]
	case "openai":
		rec["openai_api_key"] = settings["openai_api_key"]
		mo := settings["openai_model"]
		if mo == "" {
			mo = "gpt-4o"
		}
		rec["openai_model"] = mo
	}

	keys := make([]string, 0, len(rec))
	for k := range rec {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([][]string, len(keys))
	for i, k := range keys {
		pairs[i] = []string{k, rec[k]}
	}
	b, _ := json.Marshal(pairs)
	return string(b)
}

func (a *App) persistConnectionTest(ok bool, message, fingerprint string) {
	if a.store == nil {
		return
	}
	p := connectionTestPersisted{
		OK:          ok,
		Message:     message,
		At:          time.Now().UnixMilli(),
		Fingerprint: fingerprint,
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = a.store.SetSetting(connectionTestSettingKey, string(raw))
}

func (a *App) clearPersistedConnectionTest() {
	if a.store == nil {
		return
	}
	_ = a.store.DeleteSetting(connectionTestSettingKey)
}

func settingsKeyInvalidatesConnectionTest(key string) bool {
	switch key {
	case "provider_type",
		"azure_endpoint",
		"azure_deployment",
		"azure_tenant_id",
		"azure_api_key",
		"openai_api_key",
		"openai_model",
		"custom_system_prompt",
		"welcome_message",
		"design_light_base_color",
		"design_dark_base_color",
		"design_ui_font_size_px",
		"design_message_font_size_px":
		return true
	default:
		return false
	}
}

func connectionTestFailureMessageForStore(apiReturn string) string {
	s := strings.TrimSpace(apiReturn)
	s = strings.TrimPrefix(s, "Error:")
	return strings.TrimSpace(s)
}
