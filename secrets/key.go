package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// KeySource supplies the key secrets are sealed with, creating it on first use.
type KeySource interface {
	Key() ([]byte, error)
}

// ErrNotFound is what a Keychain returns for an entry it does not have.
var ErrNotFound = errors.New("secrets: not in the keychain")

// Keychain stores small secrets in the operating system's credential store.
type Keychain interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
}

// OSKeychain is the operating system's credential store: the macOS Keychain, the Windows
// Credential Manager, or the Secret Service (GNOME Keyring, KWallet, KeePassXC) on Linux.
func OSKeychain() Keychain { return osKeychain{} }

type osKeychain struct{}

func (osKeychain) Get(service, account string) (string, error) {
	v, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (osKeychain) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

// keychainKeySource keeps the key in the keychain. Where there is no keychain (e.g. Linux
// without a Secret Service running) it keeps the key in a file only the user can read, apart
// from the database, so a copy of the database alone does not reveal the secrets.
type keychainKeySource struct {
	keychain     Keychain
	service      string
	account      string
	fallbackFile string
}

// NewKeychainKeySource keeps the key under service/account in keychain, or in fallbackFile
// when the keychain cannot store it.
func NewKeychainKeySource(keychain Keychain, service, account, fallbackFile string) KeySource {
	return keychainKeySource{keychain: keychain, service: service, account: account, fallbackFile: fallbackFile}
}

func (s keychainKeySource) Key() ([]byte, error) {
	// A key that once had to go to the file stays there, so the keychain becoming available
	// later does not replace it (and lose what it sealed).
	if data, err := os.ReadFile(s.fallbackFile); err == nil {
		return decodeKey(string(data))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("secrets: read key file: %w", err)
	}

	stored, err := s.keychain.Get(s.service, s.account)
	if err == nil {
		return decodeKey(stored)
	}

	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if errors.Is(err, ErrNotFound) {
		if setErr := s.keychain.Set(s.service, s.account, encoded); setErr == nil {
			return key, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.fallbackFile), 0o700); err != nil {
		return nil, fmt.Errorf("secrets: create key folder: %w", err)
	}
	if err := os.WriteFile(s.fallbackFile, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("secrets: write key file: %w", err)
	}
	return key, nil
}

func decodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("secrets: stored key is corrupt: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("secrets: stored key is %d bytes, want %d", len(key), KeySize)
	}
	return key, nil
}
