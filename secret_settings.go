package main

import (
	"fmt"
	"path/filepath"

	"pedro/secrets"
)

// secretSettingKeys are the settings that hold credentials: the API keys, and the last
// connection test, whose fingerprint contains the key it was made with.
var secretSettingKeys = map[string]bool{
	"azure_api_key":          true,
	"openai_api_key":         true,
	"compat_api_key":         true,
	connectionTestSettingKey: true,
}

// encryptedSecretsStore keeps secret settings sealed in the database, so reading the database
// file does not reveal the API keys. Everything else goes to the wrapped Store unchanged.
type encryptedSecretsStore struct {
	Store
	box secrets.Box
}

// withEncryptedSecrets seals the secret settings of store with a key kept in the operating
// system's keychain (or a private file where there is none). Should no key be available, the
// settings stay as they are rather than becoming unusable.
func withEncryptedSecrets(store Store) Store {
	keys := secrets.NewKeychainKeySource(secrets.OSKeychain(), "Pedro", "settings-encryption-key", filepath.Join(appDataDir(), "settings.key"))
	key, err := keys.Key()
	if err != nil {
		fmt.Println("Warning: API keys are stored unencrypted:", err.Error())
		return store
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		fmt.Println("Warning: API keys are stored unencrypted:", err.Error())
		return store
	}
	return newEncryptedSecretsStore(store, box)
}

// newEncryptedSecretsStore wraps store and seals any secret setting still stored in plain text.
func newEncryptedSecretsStore(store Store, box secrets.Box) *encryptedSecretsStore {
	s := &encryptedSecretsStore{Store: store, box: box}
	s.sealPlaintextSecrets()
	return s
}

func (s *encryptedSecretsStore) GetSetting(key string) (string, error) {
	v, err := s.Store.GetSetting(key)
	if err != nil {
		return v, err
	}
	return s.open(key, v), nil
}

func (s *encryptedSecretsStore) SetSetting(key, value string) error {
	if !secretSettingKeys[key] || value == "" {
		return s.Store.SetSetting(key, value)
	}
	sealed, err := s.box.Seal(value)
	if err != nil {
		return fmt.Errorf("encrypt %s: %w", key, err)
	}
	return s.Store.SetSetting(key, sealed)
}

func (s *encryptedSecretsStore) GetSettings() (map[string]string, error) {
	settings, err := s.Store.GetSettings()
	if err != nil {
		return settings, err
	}
	for key, v := range settings {
		settings[key] = s.open(key, v)
	}
	return settings, nil
}

// open returns a secret setting's value. A value that cannot be opened (its key was lost with
// the keychain) reads as empty, so the person is asked for it again.
func (s *encryptedSecretsStore) open(key, v string) string {
	if !secretSettingKeys[key] || !secrets.IsSealed(v) {
		return v
	}
	plain, err := s.box.Open(v)
	if err != nil {
		fmt.Printf("Warning: cannot decrypt setting %s, it must be entered again: %v\n", key, err)
		return ""
	}
	return plain
}

// sealPlaintextSecrets encrypts secrets saved before encryption existed.
func (s *encryptedSecretsStore) sealPlaintextSecrets() {
	settings, err := s.Store.GetSettings()
	if err != nil {
		return
	}
	for key := range secretSettingKeys {
		if v := settings[key]; v != "" && !secrets.IsSealed(v) {
			if err := s.SetSetting(key, v); err != nil {
				fmt.Println("Warning: failed to encrypt setting:", err.Error())
			}
		}
	}
}
