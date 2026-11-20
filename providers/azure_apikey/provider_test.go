package azure_apikey

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pedro/media"
)

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("empty config should fail")
	}
	if err := (Config{
		Endpoint:   "https://x.openai.azure.com",
		Deployment: "d",
		APIKey:     "k",
	}).Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

func TestParseConfigDefaultAPIVersion(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"azure_endpoint":   "https://res.openai.azure.com",
		"azure_deployment": "dep",
		"azure_api_key":    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := cfg.(Config)
	if !ok {
		t.Fatalf("expected Config, got %T", cfg)
	}
	if c.APIVersion != DefaultAPIVersion {
		t.Fatalf("API version: got %q", c.APIVersion)
	}
	if c.Type() != "azure_apikey" {
		t.Fatalf("Type: got %q", c.Type())
	}
}

type wrongAPIKeyConfig struct{}

func (wrongAPIKeyConfig) Type() string    { return "other" }
func (wrongAPIKeyConfig) Validate() error { return nil }

func TestBuildRejectsWrongConfigType(t *testing.T) {
	_, err := Build(wrongAPIKeyConfig{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}

func TestParseConfigMediaDeployments(t *testing.T) {
	base := map[string]string{
		"azure_endpoint":   "https://res.openai.azure.com",
		"azure_deployment": "dep",
		"azure_api_key":    "secret",
	}

	cfg, _ := ParseConfig(base)
	c := cfg.(Config)
	if c.ImageDeployment != DefaultImageDeployment || c.TranscriptionDeployment != DefaultTranscriptionDeployment {
		t.Fatalf("defaults not applied: %+v", c)
	}

	base["azure_image_deployment"] = "  my-image  "
	base["azure_transcription_deployment"] = ""
	cfg, _ = ParseConfig(base)
	c = cfg.(Config)
	if c.ImageDeployment != "my-image" || c.TranscriptionDeployment != "" {
		t.Fatalf("explicit settings not honoured: %+v", c)
	}

	client, err := Build(c, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	services := client.(*Provider).MediaServices()
	if services.Images == nil {
		t.Fatal("image generator should be configured")
	}
	if services.Transcriber != nil {
		t.Fatal("empty transcription deployment should disable transcription")
	}
}

// TestMediaRequestsUseDeploymentRoutes checks the URLs sent to Azure: every
// media call must target its deployment with the media API version.
func TestMediaRequestsUseDeploymentRoutes(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" key="+r.Header.Get("Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/audio/") {
			_, _ = w.Write([]byte(`{"text":"hi"}`))
			return
		}
		_, _ = w.Write([]byte(`{"created":1,"output_format":"png","data":[{"b64_json":"` + base64.StdEncoding.EncodeToString([]byte("png")) + `"}]}`))
	}))
	defer srv.Close()

	cfg, _ := ParseConfig(map[string]string{
		"azure_endpoint":                 srv.URL,
		"azure_deployment":               "chat",
		"azure_api_key":                  "secret",
		"azure_image_deployment":         "img",
		"azure_transcription_deployment": "stt",
	})
	client, err := Build(cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	services := client.(*Provider).MediaServices()
	ctx := context.Background()

	if _, err := services.Images.GenerateImage(ctx, media.ImageRequest{Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.Images.GenerateImage(ctx, media.ImageRequest{Prompt: "p", Sources: []media.SourceImage{{Data: []byte("x"), Filename: "a.png"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.Transcriber.Transcribe(ctx, media.TranscriptionRequest{Audio: strings.NewReader("a"), Filename: "a.mp3"}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"POST /openai/deployments/img/images/generations?api-version=" + MediaAPIVersion + " key=secret",
		"POST /openai/deployments/img/images/edits?api-version=" + MediaAPIVersion + " key=secret",
		"POST /openai/deployments/stt/audio/transcriptions?api-version=" + MediaAPIVersion + " key=secret",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
