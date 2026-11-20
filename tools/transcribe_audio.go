package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pedro/media"
)

const (
	TranscribeAudioToolName = "transcribe_audio"
	transcribeTimeout       = 10 * time.Minute
	// transcribeMaxBytes is the upload limit of the OpenAI / Azure transcription API.
	transcribeMaxBytes = 25 << 20
)

// supportedAudioExts lists the formats accepted by the transcription API.
var supportedAudioExts = map[string]bool{
	".mp3": true, ".mp4": true, ".mpeg": true, ".mpga": true, ".m4a": true,
	".wav": true, ".webm": true, ".ogg": true, ".oga": true, ".flac": true,
}

// TranscribeAudioTool transcribes a local audio file with the active
// provider's transcription model.
type TranscribeAudioTool struct {
	source media.Source
}

func NewTranscribeAudioTool(source media.Source) *TranscribeAudioTool {
	return &TranscribeAudioTool{source: source}
}

func (t *TranscribeAudioTool) Definition() Definition {
	return Definition{
		Name:         TranscribeAudioToolName,
		Description:  "Transcribe a local audio file (mp3, m4a, wav, webm, ogg, flac, mp4, mpeg) to text with the configured transcription model. Max 25 MB per file.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the audio file",
				},
				"language": map[string]any{
					"type":        "string",
					"description": "Optional ISO-639-1 language code of the speech (e.g. 'en', 'de') to improve accuracy",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Optional context to guide the transcription, e.g. names, acronyms or technical terms spoken in the audio",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *TranscribeAudioTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Prompt   string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	path := filepath.Clean(strings.TrimSpace(args.Path))
	if !filepath.IsAbs(path) {
		return "", errors.New("path must be absolute")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !supportedAudioExts[ext] {
		return "", fmt.Errorf("unsupported audio format %q (supported: mp3, mp4, mpeg, mpga, m4a, wav, webm, ogg, flac)", ext)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("path is a directory")
	}
	if info.Size() > transcribeMaxBytes {
		return "", fmt.Errorf("file is %.1f MB; the transcription limit is 25 MB — split or compress the audio first", float64(info.Size())/(1<<20))
	}

	transcriber := t.source.Current().Transcriber
	if transcriber == nil {
		return "", errors.New("audio transcription is not configured: choose the Azure OpenAI (API Key) provider and set a transcription deployment in Settings")
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), transcribeTimeout)
	defer cancel()
	text, err := transcriber.Transcribe(ctx, media.TranscriptionRequest{
		Audio:    f,
		Filename: filepath.Base(path),
		Language: strings.TrimSpace(args.Language),
		Prompt:   strings.TrimSpace(args.Prompt),
	})
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Sprintf("Transcript of %s: (no speech detected)", filepath.Base(path)), nil
	}
	return fmt.Sprintf("Transcript of %s:\n\n%s", filepath.Base(path), text), nil
}
