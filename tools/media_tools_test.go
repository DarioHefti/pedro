package tools

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pedro/media"
)

type fakeImageGenerator struct{ got media.ImageRequest }

func (f *fakeImageGenerator) GenerateImage(_ context.Context, req media.ImageRequest) (media.Image, error) {
	f.got = req
	return media.Image{Data: []byte("img"), Format: "png", RevisedPrompt: "revised"}, nil
}

type fakeTranscriber struct {
	got   media.TranscriptionRequest
	audio string
}

func (f *fakeTranscriber) Transcribe(_ context.Context, req media.TranscriptionRequest) (string, error) {
	f.got = req
	data, _ := io.ReadAll(req.Audio)
	f.audio = string(data)
	return " hello world ", nil
}

func switchWith(s media.Services) *media.Switch {
	sw := media.NewSwitch()
	sw.Set(s)
	return sw
}

func TestGenerateImageToolReturnsMarkdown(t *testing.T) {
	gen := &fakeImageGenerator{}
	dir := t.TempDir()
	tool := NewGenerateImageTool(switchWith(media.Services{Images: gen}), media.NewDirImageStore(dir, media.GeneratedImagesRoute))

	out, err := tool.Execute(`{"prompt":"a [red] parrot (cartoon)","size":"1024x1024","quality":"low"}`)
	if err != nil {
		t.Fatal(err)
	}
	if gen.got.Prompt != "a [red] parrot (cartoon)" || gen.got.Size != "1024x1024" || gen.got.Quality != "low" {
		t.Fatalf("request not forwarded: %+v", gen.got)
	}
	if !strings.Contains(out, "![a red parrot cartoon]("+media.GeneratedImagesRoute) {
		t.Fatalf("markdown missing or alt not sanitized:\n%s", out)
	}
	if !strings.Contains(out, dir) || !strings.Contains(out, "revised") {
		t.Fatalf("path or revised prompt missing:\n%s", out)
	}
}

func TestGenerateImageToolEditsSourceImages(t *testing.T) {
	gen := &fakeImageGenerator{}
	store := media.NewDirImageStore(t.TempDir(), media.GeneratedImagesRoute)
	tool := NewGenerateImageTool(switchWith(media.Services{Images: gen}), store)

	attached := filepath.Join(t.TempDir(), "poster.JPG")
	if err := os.WriteFile(attached, []byte("jpg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	earlier, err := store.Save(media.Image{Data: []byte("png-bytes"), Format: "png"})
	if err != nil {
		t.Fatal(err)
	}

	// The earlier image is referenced by its chat URL, which the tool resolves.
	out, err := tool.Execute(`{"prompt":"replace the title with bla","images":[` + jsonString(attached) + `,` + jsonString(earlier.URL) + `]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.got.Sources) != 2 ||
		gen.got.Sources[0].Filename != "poster.JPG" || string(gen.got.Sources[0].Data) != "jpg-bytes" ||
		string(gen.got.Sources[1].Data) != "png-bytes" {
		t.Fatalf("sources not forwarded: %+v", gen.got.Sources)
	}
	if path, ok := GeneratedImagePath(out); !ok || filepath.Dir(path) != filepath.Dir(earlier.Path) {
		t.Fatalf("edited image path not parseable from:\n%s", out)
	}
}

func TestGeneratedImagePath(t *testing.T) {
	store := media.NewDirImageStore(t.TempDir(), media.GeneratedImagesRoute)
	tool := NewGenerateImageTool(switchWith(media.Services{Images: &fakeImageGenerator{}}), store)
	out, err := tool.Execute(`{"prompt":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	path, ok := GeneratedImagePath(out)
	if !ok {
		t.Fatalf("no path in:\n%s", out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("parsed path %q does not exist", path)
	}
	for _, other := range []string{"", "Tool generate_image error: boom", "Transcript of a.mp3:\n\nImage generated and saved to: /x.png"} {
		if _, ok := GeneratedImagePath(other); ok {
			t.Errorf("%q should not yield a path", other)
		}
	}
}

func TestGenerateImageToolRejectsBadSources(t *testing.T) {
	dir := t.TempDir()
	gif := filepath.Join(dir, "anim.gif")
	_ = os.WriteFile(gif, []byte("x"), 0o644)
	tool := NewGenerateImageTool(switchWith(media.Services{Images: &fakeImageGenerator{}}), media.NewDirImageStore(dir, media.GeneratedImagesRoute))

	for name, images := range map[string]string{
		"relative":    `["poster.png"]`,
		"unsupported": `[` + jsonString(gif) + `]`,
		"missing":     `[` + jsonString(filepath.Join(dir, "nope.png")) + `]`,
		"too many":    `[` + strings.TrimSuffix(strings.Repeat(jsonString(gif)+",", maxSourceImages+1), ",") + `]`,
	} {
		if _, err := tool.Execute(`{"prompt":"x","images":` + images + `}`); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestGenerateImageToolNotConfigured(t *testing.T) {
	tool := NewGenerateImageTool(media.NewSwitch(), media.NewDirImageStore(t.TempDir(), media.GeneratedImagesRoute))
	if _, err := tool.Execute(`{"prompt":"x"}`); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got %v", err)
	}
	if _, err := tool.Execute(`{"prompt":"  "}`); err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestImageAltTextTruncates(t *testing.T) {
	alt := imageAltText(strings.Repeat("ab ", 100))
	if len([]rune(alt)) > imageAltMaxLen+3 || !strings.HasSuffix(alt, "...") {
		t.Fatalf("alt not truncated: %q", alt)
	}
}

func TestTranscribeAudioTool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memo.mp3")
	if err := os.WriteFile(path, []byte("audio-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := &fakeTranscriber{}
	tool := NewTranscribeAudioTool(switchWith(media.Services{Transcriber: tr}))

	out, err := tool.Execute(`{"path":` + jsonString(path) + `,"language":"de"}`)
	if err != nil {
		t.Fatal(err)
	}
	if tr.got.Filename != "memo.mp3" || tr.got.Language != "de" || tr.audio != "audio-bytes" {
		t.Fatalf("request not forwarded: %+v audio=%q", tr.got, tr.audio)
	}
	if out != "Transcript of memo.mp3:\n\nhello world" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestTranscribeAudioToolValidation(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(txt, []byte("x"), 0o644)
	big := filepath.Join(dir, "big.mp3")
	f, _ := os.Create(big)
	_ = f.Truncate(transcribeMaxBytes + 1)
	f.Close()
	ok := filepath.Join(dir, "ok.wav")
	_ = os.WriteFile(ok, []byte("x"), 0o644)

	configured := NewTranscribeAudioTool(switchWith(media.Services{Transcriber: &fakeTranscriber{}}))
	for name, args := range map[string]string{
		"relative":    `{"path":"memo.mp3"}`,
		"unsupported": `{"path":` + jsonString(txt) + `}`,
		"missing":     `{"path":` + jsonString(filepath.Join(dir, "nope.mp3")) + `}`,
		"too large":   `{"path":` + jsonString(big) + `}`,
	} {
		if _, err := configured.Execute(args); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	unconfigured := NewTranscribeAudioTool(media.NewSwitch())
	if _, err := unconfigured.Execute(`{"path":` + jsonString(ok) + `}`); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got %v", err)
	}
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
