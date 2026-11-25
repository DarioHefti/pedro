package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBox(t *testing.T, fill byte) Box {
	t.Helper()
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = fill
	}
	box, err := NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestBoxSealsAndOpens(t *testing.T) {
	box := testBox(t, 1)
	sealed, err := box.Seal("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealed(sealed) || strings.Contains(sealed, "sk-secret") {
		t.Fatalf("sealed value %q is readable", sealed)
	}
	again, _ := box.Seal("sk-secret")
	if again == sealed {
		t.Fatal("sealing the same value twice gave the same text; the nonce is not random")
	}
	if plain, err := box.Open(sealed); err != nil || plain != "sk-secret" {
		t.Fatalf("Open = %q, %v", plain, err)
	}
}

func TestBoxRejectsOtherKeysAndTampering(t *testing.T) {
	sealed, _ := testBox(t, 1).Seal("sk-secret")
	if _, err := testBox(t, 2).Open(sealed); err == nil {
		t.Fatal("a different key opened the value")
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := testBox(t, 1).Open(tampered); err == nil {
		t.Fatal("a changed value opened")
	}
	if _, err := testBox(t, 1).Open("sk-plain"); err == nil {
		t.Fatal("an unsealed value opened")
	}
	if _, err := NewBox([]byte("short")); err == nil {
		t.Fatal("a short key was accepted")
	}
}

type fakeKeychain struct {
	entries map[string]string
	getErr  error
	setErr  error
}

func (k *fakeKeychain) Get(service, account string) (string, error) {
	if k.getErr != nil {
		return "", k.getErr
	}
	v, ok := k.entries[service+"/"+account]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (k *fakeKeychain) Set(service, account, secret string) error {
	if k.setErr != nil {
		return k.setErr
	}
	k.entries[service+"/"+account] = secret
	return nil
}

func TestKeySourceKeepsTheKeyInTheKeychain(t *testing.T) {
	kc := &fakeKeychain{entries: map[string]string{}}
	file := filepath.Join(t.TempDir(), "settings.key")
	src := NewKeychainKeySource(kc, "Pedro", "key", file)

	first, err := src.Key()
	if err != nil || len(first) != KeySize {
		t.Fatalf("Key = %v, %v", first, err)
	}
	second, _ := src.Key()
	if string(first) != string(second) {
		t.Fatal("the key changed between calls")
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the key was written to the fallback file although the keychain works")
	}
}

func TestKeySourceFallsBackToAPrivateFile(t *testing.T) {
	kc := &fakeKeychain{entries: map[string]string{}, getErr: errors.New("no secret service")}
	file := filepath.Join(t.TempDir(), "sub", "settings.key")
	src := NewKeychainKeySource(kc, "Pedro", "key", file)

	first, err := src.Key()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("no fallback file: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("key file permissions %v let others read it", perm)
	}

	// Once in the file, the key stays there even when the keychain starts working.
	kc.getErr = nil
	second, err := src.Key()
	if err != nil || string(first) != string(second) {
		t.Fatalf("key changed after the keychain came back: %v", err)
	}
	if len(kc.entries) != 0 {
		t.Fatal("a second key went into the keychain")
	}
}

func TestKeySourceFallsBackWhenTheKeychainCannotStore(t *testing.T) {
	kc := &fakeKeychain{entries: map[string]string{}, setErr: errors.New("locked")}
	file := filepath.Join(t.TempDir(), "settings.key")
	if _, err := NewKeychainKeySource(kc, "Pedro", "key", file).Key(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("no fallback file: %v", err)
	}
}
