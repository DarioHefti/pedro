package tools

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"pedro/media"
)

type splitterFunc func(context.Context, io.Reader, func(io.Reader) error) error

func (f splitterFunc) Split(ctx context.Context, r io.Reader, emit func(io.Reader) error) error {
	return f(ctx, r, emit)
}

type transcriberFunc func(context.Context, media.TranscriptionRequest) (string, error)

func (f transcriberFunc) Transcribe(ctx context.Context, req media.TranscriptionRequest) (string, error) {
	return f(ctx, req)
}

func TestTranscribeAudioToolLargeMP3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long.MP3")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, transcribeMaxBytes+1); err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("語", 3000)
	var files []string
	tr := transcriberFunc(func(ctx context.Context, req media.TranscriptionRequest) (string, error) {
		files = append(files, req.Filename)
		data, err := io.ReadAll(req.Audio)
		if err != nil || string(data) != req.Filename || req.Language != "de" || req.Prompt != "names" {
			t.Fatalf("bad upload: %+v, audio %q, error %v", req, data, err)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("upload did not get its own timeout")
		}
		if len(files) == 1 {
			return text, nil
		}
		return "last chunk", nil
	})
	savedDir := filepath.Join(dir, "transcripts")
	tool := NewTranscribeAudioTool(switchWith(media.Services{Transcriber: tr}), media.NewDirTranscriptStore(savedDir))
	tool.splitter = splitterFunc(func(_ context.Context, _ io.Reader, emit func(io.Reader) error) error {
		if err := emit(strings.NewReader("chunk-0001.wav")); err != nil {
			return err
		}
		return emit(strings.NewReader("chunk-0002.wav"))
	})
	out, err := tool.ExecuteContext(openFiles(t), `{"path":`+jsonString(path)+`,"language":" de ","prompt":" names "}`)
	if err != nil || len(files) != 2 {
		t.Fatalf("uploads %v, error %v", files, err)
	}
	entries, err := os.ReadDir(savedDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("saved transcripts %v, error %v", entries, err)
	}
	saved := filepath.Join(savedDir, entries[0].Name())
	if !strings.Contains(out, saved) || !strings.Contains(out, "Preview:") || !utf8.ValidString(out) || strings.Contains(out, "last chunk") {
		t.Fatalf("unexpected tool output: %q", out)
	}
	data, err := os.ReadFile(saved)
	if err != nil || strings.ReplaceAll(string(data), "\n", "") != text+"last chunk" {
		t.Fatalf("saved file size %d, err %v, or content out of order", len(data), err)
	}
	page, err := readTextFile(saved, 1, 20)
	if err != nil || strings.Contains(page, "truncated") || !strings.Contains(page, "last chunk") {
		t.Fatalf("full transcript is not readable: %v", err)
	}
}

func TestTranscribeAudioToolCancellationStopsChunks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var uploads int
	tr := transcriberFunc(func(reqCtx context.Context, _ media.TranscriptionRequest) (string, error) {
		uploads++
		cancel()
		return "", reqCtx.Err()
	})
	tool := NewTranscribeAudioTool(switchWith(media.Services{Transcriber: tr}), media.NewDirTranscriptStore(dir))
	tool.splitter = splitterFunc(func(_ context.Context, _ io.Reader, emit func(io.Reader) error) error {
		if err := emit(strings.NewReader("first")); err != nil {
			return err
		}
		return emit(strings.NewReader("second"))
	})
	_, err := tool.ExecuteContext(WithFileAccess(ctx, allowEverything{}), `{"path":`+jsonString(path)+`}`)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "chunk 1") || uploads != 1 {
		t.Fatalf("uploads %d, error %v", uploads, err)
	}
}
