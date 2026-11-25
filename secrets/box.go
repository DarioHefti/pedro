// Package secrets keeps credentials, such as API keys, unreadable at rest.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// KeySize is the length of the key a Box needs.
const KeySize = 32

// sealedPrefix marks a sealed value and its format version.
const sealedPrefix = "enc:v1:"

// Box seals values with a secret key and opens them again.
type Box interface {
	Seal(plain string) (string, error)
	Open(sealed string) (string, error)
}

// IsSealed reports whether v is a value a Box sealed.
func IsSealed(v string) bool {
	return strings.HasPrefix(v, sealedPrefix)
}

// aesBox seals with AES-256-GCM: a random nonce per value, and tampering is detected.
type aesBox struct {
	aead cipher.AEAD
}

// NewBox returns a Box that seals with key, which must be KeySize bytes.
func NewBox(key []byte) (Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secrets: key is %d bytes, want %d", len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aesBox{aead: aead}, nil
}

func (b aesBox) Seal(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return sealedPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (b aesBox) Open(sealed string) (string, error) {
	encoded, ok := strings.CutPrefix(sealed, sealedPrefix)
	if !ok {
		return "", errors.New("secrets: value is not sealed")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("secrets: %w", err)
	}
	n := b.aead.NonceSize()
	if len(data) < n {
		return "", errors.New("secrets: sealed value is too short")
	}
	plain, err := b.aead.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return "", errors.New("secrets: cannot open the value; it was sealed with another key or changed")
	}
	return string(plain), nil
}
