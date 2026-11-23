//go:build live

package azure_apikey

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pedro/media"
	"pedro/shared"
	"pedro/tools"
)

// Live tests against a real Azure OpenAI resource. Run with:
//
//	AZURE_OPENAI_ENDPOINT=https://<res>.openai.azure.com AZURE_OPENAI_API_KEY=... \
//	AZURE_OPENAI_DEPLOYMENT=<chat deployment> PEDRO_LIVE_AUDIO=/abs/path/speech.mp3 \
//	go test -tags live -run Live -v ./providers/azure_apikey/
//
// Set PEDRO_LIVE_OUT to a directory to keep the generated images.
//
// Image/transcription deployments default to DefaultImageDeployment and
// DefaultTranscriptionDeployment; override with AZURE_OPENAI_IMAGE_DEPLOYMENT
// and AZURE_OPENAI_TRANSCRIPTION_DEPLOYMENT.
func liveProvider(t *testing.T, registry *tools.Registry) *Provider {
	t.Helper()
	settings := map[string]string{
		"azure_endpoint":   os.Getenv("AZURE_OPENAI_ENDPOINT"),
		"azure_api_key":    os.Getenv("AZURE_OPENAI_API_KEY"),
		"azure_deployment": os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
	}
	if settings["azure_endpoint"] == "" || settings["azure_api_key"] == "" {
		t.Skip("AZURE_OPENAI_ENDPOINT / AZURE_OPENAI_API_KEY not set")
	}
	if v, ok := os.LookupEnv("AZURE_OPENAI_IMAGE_DEPLOYMENT"); ok {
		settings["azure_image_deployment"] = v
	}
	if v, ok := os.LookupEnv("AZURE_OPENAI_TRANSCRIPTION_DEPLOYMENT"); ok {
		settings["azure_transcription_deployment"] = v
	}
	cfg, _ := ParseConfig(settings)
	client, err := Build(cfg, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	return client.(*Provider)
}

func liveAudio(t *testing.T) string {
	t.Helper()
	path := os.Getenv("PEDRO_LIVE_AUDIO")
	if path == "" {
		t.Skip("PEDRO_LIVE_AUDIO not set")
	}
	return path
}

func TestLiveGenerateImage(t *testing.T) {
	p := liveProvider(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	img, err := p.MediaServices().Images.GenerateImage(ctx, media.ImageRequest{
		Prompt:  "A friendly cartoon parrot wearing headphones, flat illustration",
		Size:    "1024x1024",
		Quality: "low",
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := media.NewDirImageStore(t.TempDir(), media.GeneratedImagesRoute).Save(img)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated %s image (%d KB) in %s -> %s", img.Format, len(img.Data)/1024, time.Since(start).Round(time.Millisecond), stored.Path)
}

func TestLiveTranscribe(t *testing.T) {
	path := liveAudio(t)
	p := liveProvider(t, nil)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	text, err := p.MediaServices().Transcriber.Transcribe(context.Background(), media.TranscriptionRequest{
		Audio:    f,
		Filename: filepath.Base(path),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatal("empty transcript")
	}
	t.Logf("transcript: %s", text)
}

// TestLiveChatUsesMediaTools checks the whole backend flow: the chat model
// discovers the tools via tool_search, calls them, and answers with the result.
func TestLiveChatUsesMediaTools(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_DEPLOYMENT") == "" {
		t.Skip("AZURE_OPENAI_DEPLOYMENT not set")
	}
	sw := media.NewSwitch()
	imageDir := os.Getenv("PEDRO_LIVE_OUT")
	if imageDir == "" {
		imageDir = t.TempDir()
	}
	registry := tools.New(noMemory{}, sw, media.NewDirImageStore(imageDir, media.GeneratedImagesRoute), media.NewDirTranscriptStore(t.TempDir()))
	defer registry.Close()
	p := liveProvider(t, registry)
	sw.Set(p.MediaServices())

	// chat returns the final reply and the called tools with their arguments.
	chat := func(prompt string, imageDataURLs ...string) (string, map[string]string) {
		t.Helper()
		called := map[string]string{}
		msgs, err := p.Chat(context.Background(), []shared.Message{{Role: "user", Content: prompt}}, imageDataURLs, nil,
			func(name, args, _ string) {
				called[name] = args
				t.Logf("tool call: %s %s", name, args)
			}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		final := msgs[len(msgs)-1].Content
		t.Logf("reply:\n%s", final)
		return final, called
	}

	t.Run("image", func(t *testing.T) {
		reply, called := chat("Please generate a small square image of a cartoon parrot drinking coffee. Use low quality.")
		if _, ok := called[tools.GenerateImageToolName]; !ok {
			t.Fatalf("generate_image not called: %v", called)
		}
		if !strings.Contains(reply, "]("+media.GeneratedImagesRoute) {
			t.Fatalf("reply does not embed the generated image")
		}
		entries, _ := os.ReadDir(imageDir)
		if len(entries) == 0 {
			t.Fatal("no image stored")
		}
	})

	t.Run("transcription", func(t *testing.T) {
		path := liveAudio(t)
		// Same content the frontend builds for an attached audio file.
		reply, called := chat("What is said in this recording?\n\n[File: " + filepath.Base(path) + "]\n[Path: " + path + "]\n[Audio file attached by path. Use the transcribe_audio tool with this path to transcribe it.]")
		if _, ok := called[tools.TranscribeAudioToolName]; !ok {
			t.Fatalf("transcribe_audio not called: %v", called)
		}
		if strings.TrimSpace(reply) == "" {
			t.Fatal("empty reply")
		}
	})

	// A poster with a title to edit, as the user would attach it.
	poster := func(t *testing.T) (path, dataURL string) {
		t.Helper()
		img, err := p.MediaServices().Images.GenerateImage(context.Background(), media.ImageRequest{
			Prompt:  "A simple flat event poster. Large bold title text at the top reading exactly: SUMMER PARTY. Below it a cartoon parrot with sunglasses.",
			Size:    "1024x1024",
			Quality: "low",
		})
		if err != nil {
			t.Fatal(err)
		}
		stored, err := media.NewDirImageStore(imageDir, media.PastedImagesRoute).Save(img)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("poster: %s", stored.Path)
		return stored.Path, "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Data)
	}

	assertEdited := func(t *testing.T, reply string, called map[string]string, source string) {
		t.Helper()
		args, ok := called[tools.GenerateImageToolName]
		if !ok {
			t.Fatalf("generate_image not called: %v", called)
		}
		if !strings.Contains(args, `"images"`) || !strings.Contains(args, filepath.Base(source)) {
			t.Fatalf("generate_image was not given the source image: %s", args)
		}
		if !strings.Contains(reply, "]("+media.GeneratedImagesRoute) {
			t.Fatalf("reply does not embed the edited image")
		}
	}

	t.Run("edit attached image", func(t *testing.T) {
		path, dataURL := poster(t)
		// Same content the frontend builds for a dropped image file (pixels attached for vision).
		reply, called := chat("Replace the title with BLA\n\n[File: "+filepath.Base(path)+"]\n[Path: "+path+"]\n[Raster image attached by path; the image pixels are also included in this message for vision.]", dataURL)
		assertEdited(t, reply, called, path)
	})

	t.Run("edit pasted image", func(t *testing.T) {
		path, dataURL := poster(t)
		// Same note the backend appends for pasted images (annotatePastedImages).
		reply, called := chat("Replace the title with BLA\n\n[File: pasted image 1]\n[Image file: "+path+"]\n[Image pasted into the chat; its pixels are included for vision. To edit it, pass this path to generate_image.]", dataURL)
		assertEdited(t, reply, called, path)
	})
}

type noMemory struct{}

func (noMemory) GetMemories() ([]shared.MemoryRecord, error)          { return nil, nil }
func (noMemory) GetMemoryKeys() ([]string, error)                     { return nil, nil }
func (noMemory) SearchMemories(string) ([]shared.MemoryRecord, error) { return nil, nil }
func (noMemory) SaveMemory(_, _, _ string) error                      { return nil }
func (noMemory) SaveMemoryWithMeta(_, _, _, _ string, _ int) error    { return nil }
func (noMemory) ForgetMemory(int64) error                             { return nil }
