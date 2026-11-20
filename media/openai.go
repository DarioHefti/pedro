package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/openai/openai-go"
)

// maxDownloadedImageBytes caps images fetched from a URL response.
const maxDownloadedImageBytes = 50 << 20

// OpenAIImageGenerator generates images through the OpenAI images API. It works
// with any openai-go client, including Azure deployments (model = deployment).
type OpenAIImageGenerator struct {
	client openai.Client
	model  string
}

func NewOpenAIImageGenerator(client openai.Client, model string) *OpenAIImageGenerator {
	return &OpenAIImageGenerator{client: client, model: model}
}

// GenerateImage uses the edits endpoint when source images are given,
// otherwise the generations endpoint.
func (g *OpenAIImageGenerator) GenerateImage(ctx context.Context, req ImageRequest) (Image, error) {
	var resp *openai.ImagesResponse
	var err error
	if len(req.Sources) > 0 {
		resp, err = g.edit(ctx, req)
	} else {
		resp, err = g.generate(ctx, req)
	}
	if err != nil {
		return Image{}, fmt.Errorf("image generation failed: %w", err)
	}
	return decodeImagesResponse(ctx, resp)
}

func (g *OpenAIImageGenerator) generate(ctx context.Context, req ImageRequest) (*openai.ImagesResponse, error) {
	params := openai.ImageGenerateParams{
		Prompt: req.Prompt,
		Model:  openai.ImageModel(g.model),
		N:      openai.Int(1),
	}
	if req.Size != "" {
		params.Size = openai.ImageGenerateParamsSize(req.Size)
	}
	if req.Quality != "" {
		params.Quality = openai.ImageGenerateParamsQuality(req.Quality)
	}
	return g.client.Images.Generate(ctx, params)
}

func (g *OpenAIImageGenerator) edit(ctx context.Context, req ImageRequest) (*openai.ImagesResponse, error) {
	files := make([]io.Reader, len(req.Sources))
	for i, src := range req.Sources {
		files[i] = openai.File(bytes.NewReader(src.Data), src.Filename, contentTypeFor(src.Filename))
	}
	params := openai.ImageEditParams{
		Image:  openai.ImageEditParamsImageUnion{OfFileArray: files},
		Prompt: req.Prompt,
		Model:  openai.ImageModel(g.model),
		N:      openai.Int(1),
	}
	if req.Size != "" {
		params.Size = openai.ImageEditParamsSize(req.Size)
	}
	if req.Quality != "" {
		params.Quality = openai.ImageEditParamsQuality(req.Quality)
	}
	return g.client.Images.Edit(ctx, params)
}

func decodeImagesResponse(ctx context.Context, resp *openai.ImagesResponse) (Image, error) {
	var err error
	if len(resp.Data) == 0 {
		return Image{}, errors.New("image generation returned no image")
	}

	first := resp.Data[0]
	format := string(resp.OutputFormat)
	var data []byte
	switch {
	case first.B64JSON != "":
		data, err = base64.StdEncoding.DecodeString(first.B64JSON)
		if err != nil {
			return Image{}, fmt.Errorf("decode generated image: %w", err)
		}
	case first.URL != "":
		data, err = download(ctx, first.URL)
		if err != nil {
			return Image{}, err
		}
	default:
		return Image{}, errors.New("image generation returned neither image data nor a URL")
	}
	if format == "" {
		format = sniffImageFormat(data)
	}

	return Image{Data: data, Format: format, RevisedPrompt: first.RevisedPrompt}, nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download generated image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download generated image: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownloadedImageBytes))
}

func contentTypeFor(filename string) string {
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func sniffImageFormat(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

// OpenAITranscriber transcribes audio through the OpenAI audio API. It works
// with any openai-go client, including Azure deployments (model = deployment).
type OpenAITranscriber struct {
	client openai.Client
	model  string
}

func NewOpenAITranscriber(client openai.Client, model string) *OpenAITranscriber {
	return &OpenAITranscriber{client: client, model: model}
}

func (t *OpenAITranscriber) Transcribe(ctx context.Context, req TranscriptionRequest) (string, error) {
	params := openai.AudioTranscriptionNewParams{
		File:  openai.File(req.Audio, req.Filename, contentTypeFor(req.Filename)),
		Model: openai.AudioModel(t.model),
	}
	if req.Language != "" {
		params.Language = openai.String(req.Language)
	}
	if req.Prompt != "" {
		params.Prompt = openai.String(req.Prompt)
	}

	resp, err := t.client.Audio.Transcriptions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("transcription failed: %w", err)
	}
	return resp.Text, nil
}
