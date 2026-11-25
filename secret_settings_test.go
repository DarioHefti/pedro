package main

import (
	"strings"
	"testing"

	"pedro/secrets"
)

func testSecretBox(t *testing.T, fill byte) secrets.Box {
	t.Helper()
	key := make([]byte, secrets.KeySize)
	for i := range key {
		key[i] = fill
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestEncryptedSecretsStore_SealsAPIKeysAtRest(t *testing.T) {
	db := openTestDatabase(t)
	store := newEncryptedSecretsStore(db, testSecretBox(t, 7))

	if err := store.SetSetting("openai_api_key", "sk-live-123"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSetting("openai_model", "gpt-4o"); err != nil {
		t.Fatal(err)
	}

	raw, _ := db.GetSetting("openai_api_key")
	if !secrets.IsSealed(raw) || strings.Contains(raw, "sk-live-123") {
		t.Fatalf("the database holds %q", raw)
	}
	if model, _ := db.GetSetting("openai_model"); model != "gpt-4o" {
		t.Fatalf("a plain setting was changed: %q", model)
	}
	if got, _ := store.GetSetting("openai_api_key"); got != "sk-live-123" {
		t.Fatalf("GetSetting = %q", got)
	}
	all, _ := store.GetSettings()
	if all["openai_api_key"] != "sk-live-123" || all["openai_model"] != "gpt-4o" {
		t.Fatalf("GetSettings = %v", all)
	}
	// Clearing a key stores it empty, not as a sealed empty string.
	if err := store.SetSetting("openai_api_key", ""); err != nil {
		t.Fatal(err)
	}
	if raw, _ := db.GetSetting("openai_api_key"); raw != "" {
		t.Fatalf("cleared key stored as %q", raw)
	}
}

func TestEncryptedSecretsStore_SealsKeysSavedBeforeEncryption(t *testing.T) {
	db := openTestDatabase(t)
	_ = db.SetSetting("compat_api_key", "lm-studio")
	_ = db.SetSetting(connectionTestSettingKey, `{"ok":true,"fingerprint":"[[\"compat_api_key\",\"lm-studio\"]]"}`)

	store := newEncryptedSecretsStore(db, testSecretBox(t, 7))
	for _, key := range []string{"compat_api_key", connectionTestSettingKey} {
		if raw, _ := db.GetSetting(key); !secrets.IsSealed(raw) {
			t.Errorf("%s still stored in plain text: %q", key, raw)
		}
	}
	if got, _ := store.GetSetting("compat_api_key"); got != "lm-studio" {
		t.Fatalf("GetSetting = %q", got)
	}
}

func TestEncryptedSecretsStore_ALostKeyReadsAsEmpty(t *testing.T) {
	db := openTestDatabase(t)
	_ = newEncryptedSecretsStore(db, testSecretBox(t, 1)).SetSetting("azure_api_key", "secret")

	other := newEncryptedSecretsStore(db, testSecretBox(t, 2))
	if got, err := other.GetSetting("azure_api_key"); err != nil || got != "" {
		t.Fatalf("GetSetting with the wrong key = %q, %v; want empty so it is entered again", got, err)
	}
}
