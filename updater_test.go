package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseAssetURLs(t *testing.T) {
	name, sums, err := releaseAssetURLs("https://github.com/DarioHefti/pedro/releases/download/v1.4.0/Pedro-macos-arm64.dmg")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Pedro-macos-arm64.dmg" || sums != "https://github.com/DarioHefti/pedro/releases/download/v1.4.0/SHA256SUMS" {
		t.Fatalf("name=%q sums=%q", name, sums)
	}

	for _, bad := range []string{
		"https://evil.example/releases/download/v1/Pedro.dmg",
		"https://github.com/someone/else/releases/download/v1/Pedro.dmg",
		"http://github.com/DarioHefti/pedro/releases/download/v1/Pedro.dmg",
		"https://github.com/DarioHefti/pedro/releases/download/v1/",
		"https://github.com/DarioHefti/pedro/releases/download/v1/../../x",
		"https://github.com/DarioHefti/pedro/releases/download/v1/a%2F..%2Fx",
		"https://github.com/DarioHefti/pedro/releases/download/",
	} {
		if _, _, err := releaseAssetURLs(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	sums, err := parseChecksums(strings.NewReader(sum + "  Pedro-linux-amd64\n" + strings.ToUpper(sum) + " *Pedro-windows-amd64.exe\nnot a line\n" + "123  short\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums["Pedro-linux-amd64"] != sum || sums["Pedro-windows-amd64.exe"] != sum {
		t.Fatalf("sums = %v", sums)
	}
	if _, err := parseChecksums(strings.NewReader("")); err == nil {
		t.Fatal("an empty checksum file was accepted")
	}
}

func TestDownloadVerified(t *testing.T) {
	payload := []byte("installer bytes")
	digest := sha256.Sum256(payload)
	good := hex.EncodeToString(digest[:])
	listed := good
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(listed + "  Pedro.dmg\n"))
		case "/Pedro.dmg":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "Pedro.dmg")

	if err := downloadVerified(t.Context(), server.URL+"/Pedro.dmg", server.URL+"/SHA256SUMS", "Pedro.dmg", dest); err != nil {
		t.Fatalf("a matching download failed: %v", err)
	}
	if data, _ := os.ReadFile(dest); string(data) != string(payload) {
		t.Fatal("the download was not written")
	}

	listed = strings.Repeat("0", 64)
	err := downloadVerified(t.Context(), server.URL+"/Pedro.dmg", server.URL+"/SHA256SUMS", "Pedro.dmg", dest)
	if err == nil || !strings.Contains(err.Error(), "not the published file") {
		t.Fatalf("a tampered download passed: %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatal("the tampered download was kept")
	}

	if err := downloadVerified(t.Context(), server.URL+"/Pedro.dmg", server.URL+"/missing", "Pedro.dmg", dest); err == nil {
		t.Fatal("a release without checksums passed")
	}
	if err := downloadVerified(t.Context(), server.URL+"/Pedro.dmg", server.URL+"/SHA256SUMS", "Other.dmg", dest); err == nil {
		t.Fatal("a file the checksums do not list passed")
	}
}
