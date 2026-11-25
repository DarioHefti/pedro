package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"pedro/media"
)

const (
	TranscribeAudioToolName = "transcribe_audio"
	transcribeTimeout       = 10 * time.Minute
	// transcribeMaxBytes is the upload limit of the OpenAI / Azure transcription API.
	transcribeMaxBytes     = 25 << 20
	transcriptInlineBytes  = 8 << 10
	transcriptPreviewBytes = 1000
)

// supportedAudioExts lists the formats accepted by the transcription API.
var supportedAudioExts = map[string]bool{
	".mp3": true, ".mp4": true, ".mpeg": true, ".mpga": true, ".m4a": true,
	".wav": true, ".webm": true, ".ogg": true, ".oga": true, ".flac": true,
}

// TranscribeAudioTool transcribes a local audio file with the active
// provider's transcription model.
type TranscribeAudioTool struct {
	source      media.Source
	splitter    media.AudioSplitter
	transcripts media.TranscriptStore
}

func NewTranscribeAudioTool(source media.Source, transcripts media.TranscriptStore) *TranscribeAudioTool {
	return &TranscribeAudioTool{source: source, splitter: media.NewMP3Splitter(), transcripts: transcripts}
}

func (t *TranscribeAudioTool) Definition() Definition {
	return Definition{
		Name:         TranscribeAudioToolName,
		Description:  "Transcribe a local audio file (mp3, m4a, wav, webm, ogg, flac, mp4, mpeg) to text. MP3s are automatically split near pauses into chunks up to five minutes. Other formats are limited to 25 MiB per file. Long transcripts return a local path and preview; use read_file to page through the full text.",
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
	return t.ExecuteContext(context.Background(), argsJSON)
}

func (t *TranscribeAudioTool) ExecuteContext(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Prompt   string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	path, err := fileAccessFrom(ctx).Resolve(args.Path)
	if err != nil {
		return "", err
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
	if !info.Mode().IsRegular() {
		return "", errors.New("path must be a regular file")
	}
	if ext != ".mp3" && info.Size() > transcribeMaxBytes {
		return "", fmt.Errorf("file is %.1f MiB; the limit for %s is 25 MiB — split the audio or convert it to MP3 first", float64(info.Size())/(1<<20), ext)
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

	name := filepath.Base(path)
	req := media.TranscriptionRequest{
		Language: strings.TrimSpace(args.Language),
		Prompt:   strings.TrimSpace(args.Prompt),
	}
	var transcript strings.Builder
	upload := func(audio io.Reader, filename string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunkCtx, cancel := context.WithTimeout(ctx, transcribeTimeout)
		defer cancel()
		req.Audio, req.Filename = audio, filename
		text, err := transcriber.Transcribe(chunkCtx, req)
		if err != nil {
			return err
		}
		if text = strings.TrimSpace(text); text != "" {
			if transcript.Len() != 0 {
				transcript.WriteString("\n\n")
			}
			transcript.WriteString(text)
		}
		return nil
	}
	if ext == ".mp3" {
		var chunk int
		err = t.splitter.Split(ctx, f, func(audio io.Reader) error {
			chunk++
			if err := upload(audio, fmt.Sprintf("chunk-%04d.wav", chunk)); err != nil {
				return fmt.Errorf("transcribe MP3 chunk %d: %w", chunk, err)
			}
			return nil
		})
		if chunk == 0 && errors.Is(err, media.ErrMP3DecodeStart) && ctx.Err() == nil && info.Size() <= transcribeMaxBytes {
			// Keep support for provider-decodable variants the local decoder cannot open.
			err = upload(io.NewSectionReader(f, 0, info.Size()), name)
		}
	} else {
		err = upload(f, name)
	}
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text := transcript.String()
	if text == "" {
		return fmt.Sprintf("Transcript of %s: (no speech detected)", name), nil
	}
	if len(text) <= transcriptInlineBytes {
		return fmt.Sprintf("Transcript of %s:\n\n%s", name, text), nil
	}
	saved, err := t.transcripts.Save(wrapTranscript(text))
	if err != nil {
		return "", fmt.Errorf("save transcript: %w", err)
	}
	preview := text[:utf8Boundary(text, transcriptPreviewBytes)]
	return fmt.Sprintf("Full transcript of %s saved to: %s\nUse read_file for the full transcript.\n\nPreview:\n%s…", name, saved, preview), nil
}

// Keep every transcript line within read_file's per-line limit so the full
// text is accessible through pagination, even if the provider emits one line.
func wrapTranscript(text string) string {
	var out strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i != 0 {
			out.WriteByte('\n')
		}
		for len(line) > readMaxLineLen {
			cut := utf8Boundary(line, readMaxLineLen)
			if space := strings.LastIndexAny(line[:cut], " \t"); space > cut/2 {
				cut = space + 1
			}
			out.WriteString(line[:cut])
			out.WriteByte('\n')
			line = line[cut:]
		}
		out.WriteString(line)
	}
	return out.String()
}

func utf8Boundary(s string, at int) int {
	for at > 0 && at < len(s) && !utf8.RuneStart(s[at]) {
		at--
	}
	return at
}
