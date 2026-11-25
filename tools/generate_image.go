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
	GenerateImageToolName = "generate_image"
	generateImageTimeout  = 5 * time.Minute
	imageAltMaxLen        = 80
	// Limits of the images edit API.
	maxSourceImages     = 16
	maxSourceImageBytes = 50 << 20
)

var supportedSourceImageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}

// First line of a successful result; GeneratedImagePath parses it back.
const (
	generatedImagePrefix = "Image generated and saved to: "
	editedImagePrefix    = "Image edited and saved to: "
)

// GeneratedImagePath returns the file a generate_image result reports as
// created, so the image can be cleaned up together with its chat.
func GeneratedImagePath(result string) (string, bool) {
	first, _, _ := strings.Cut(result, "\n")
	for _, prefix := range []string{generatedImagePrefix, editedImagePrefix} {
		if path, ok := strings.CutPrefix(first, prefix); ok && strings.TrimSpace(path) != "" {
			return strings.TrimSpace(path), true
		}
	}
	return "", false
}

// GenerateImageTool creates or edits an image with the active provider's
// image model and stores it so the chat can display it.
type GenerateImageTool struct {
	source media.Source
	store  media.ImageStore
}

func NewGenerateImageTool(source media.Source, store media.ImageStore) *GenerateImageTool {
	return &GenerateImageTool{source: source, store: store}
}

func (t *GenerateImageTool) Definition() Definition {
	return Definition{
		Name:         GenerateImageToolName,
		Description:  "Generate a new image from a text prompt, or edit existing images (change or replace text, add or remove objects, restyle, combine) by passing their paths in images. The result is saved locally and contains the markdown needed to show it to the user in the chat.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "For a new image: detailed description of subject, style, composition, colors, lighting and any text to render. For an edit: the change to make, and what must stay the same.",
				},
				"images": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional absolute paths of existing PNG, JPEG or WEBP images to edit or use as references, e.g. an image the user attached or one generated earlier. Omit to create a new image from the prompt alone.",
				},
				"size": map[string]any{
					"type":        "string",
					"enum":        []string{"auto", "1024x1024", "1536x1024", "1024x1536"},
					"description": "Image size: 1024x1024 (square), 1536x1024 (landscape), 1024x1536 (portrait). Default: auto.",
				},
				"quality": map[string]any{
					"type":        "string",
					"enum":        []string{"auto", "low", "medium", "high"},
					"description": "Rendering quality. Higher quality takes longer. Default: auto.",
				},
			},
			"required": []string{"prompt"},
		},
	}
}

func (t *GenerateImageTool) Execute(argsJSON string) (string, error) {
	return t.ExecuteContext(context.Background(), argsJSON)
}

func (t *GenerateImageTool) ExecuteContext(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Prompt  string   `json:"prompt"`
		Images  []string `json:"images"`
		Size    string   `json:"size"`
		Quality string   `json:"quality"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" {
		return "", errors.New("prompt is required")
	}

	generator := t.source.Current().Images
	if generator == nil {
		return "", errors.New("image generation is not configured: choose the Azure OpenAI (API Key) provider and set an image deployment in Settings")
	}
	sources, err := t.loadSources(fileAccessFrom(ctx), args.Images)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, generateImageTimeout)
	defer cancel()
	img, err := generator.GenerateImage(ctx, media.ImageRequest{Prompt: prompt, Sources: sources, Size: args.Size, Quality: args.Quality})
	if err != nil {
		return "", err
	}
	stored, err := t.store.Save(img)
	if err != nil {
		return "", err
	}

	prefix := generatedImagePrefix
	if len(sources) > 0 {
		prefix = editedImagePrefix
	}
	var b strings.Builder
	b.WriteString(prefix + stored.Path + "\n\n")
	b.WriteString("Show the image to the user by including this markdown line in your reply exactly as written:\n")
	fmt.Fprintf(&b, "![%s](%s)\n", imageAltText(prompt), stored.URL)
	if img.RevisedPrompt != "" {
		b.WriteString("\nRevised prompt used by the model: " + img.RevisedPrompt + "\n")
	}
	return b.String(), nil
}

// loadSources reads the images to edit. Besides absolute paths it accepts the
// URLs of images this tool generated earlier, as models sometimes pass those.
func (t *GenerateImageTool) loadSources(access FileAccess, refs []string) ([]media.SourceImage, error) {
	if len(refs) > maxSourceImages {
		return nil, fmt.Errorf("at most %d images can be edited at once", maxSourceImages)
	}
	sources := make([]media.SourceImage, 0, len(refs))
	for _, ref := range refs {
		path := strings.TrimSpace(ref)
		if stored, ok := t.store.Lookup(path); ok {
			path = stored
		}
		path, err := access.Resolve(path)
		if err != nil {
			return nil, fmt.Errorf("image %q: %w", ref, err)
		}
		if ext := strings.ToLower(filepath.Ext(path)); !supportedSourceImageExts[ext] {
			return nil, fmt.Errorf("unsupported image format %q for %s (supported: png, jpg, jpeg, webp)", ext, filepath.Base(path))
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("cannot access image: %w", err)
		}
		if info.Size() > maxSourceImageBytes {
			return nil, fmt.Errorf("%s is larger than 50 MB", filepath.Base(path))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read image: %w", err)
		}
		sources = append(sources, media.SourceImage{Data: data, Filename: filepath.Base(path)})
	}
	return sources, nil
}

// imageAltText turns a prompt into a short, markdown-safe alt text.
func imageAltText(prompt string) string {
	alt := strings.Join(strings.Fields(strings.NewReplacer("[", "", "]", "", "(", "", ")", "").Replace(prompt)), " ")
	if r := []rune(alt); len(r) > imageAltMaxLen {
		alt = strings.TrimSpace(string(r[:imageAltMaxLen])) + "..."
	}
	return alt
}
