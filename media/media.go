// Package media provides image generation and audio transcription behind
// small interfaces so tools never depend on a concrete provider.
package media

import (
	"context"
	"io"
	"sync"
)

// SourceImage is an existing image to edit or use as a reference.
type SourceImage struct {
	Data     []byte
	Filename string
}

// ImageRequest describes an image to generate. With Sources set, the model
// edits or combines those images according to Prompt instead of starting
// from scratch.
type ImageRequest struct {
	Prompt  string
	Sources []SourceImage
	// Size such as "1024x1024", "1536x1024", "1024x1536" or "auto". Empty uses the provider default.
	Size string
	// Quality such as "low", "medium", "high" or "auto". Empty uses the provider default.
	Quality string
}

// Image is a generated image.
type Image struct {
	Data []byte
	// Format is the file extension of Data without a dot, e.g. "png".
	Format        string
	RevisedPrompt string
}

// ImageGenerator creates images from text prompts and optional source images.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, req ImageRequest) (Image, error)
}

// TranscriptionRequest describes an audio file to transcribe.
type TranscriptionRequest struct {
	Audio    io.Reader
	Filename string
	// Language is an optional ISO-639-1 hint, e.g. "de".
	Language string
	// Prompt is optional context such as names or vocabulary used in the audio.
	Prompt string
}

// Transcriber turns speech into text.
type Transcriber interface {
	Transcribe(ctx context.Context, req TranscriptionRequest) (string, error)
}

// Services bundles the media capabilities of the active provider.
// A nil field means the capability is not configured.
type Services struct {
	Images      ImageGenerator
	Transcriber Transcriber
}

// Provider is implemented by LLM clients that also offer media capabilities.
type Provider interface {
	MediaServices() Services
}

// Source hands out the currently active media services.
type Source interface {
	Current() Services
}

// Switch is a Source whose services can be replaced at runtime, e.g. when
// the user changes provider settings. Safe for concurrent use.
type Switch struct {
	mu       sync.RWMutex
	services Services
}

func NewSwitch() *Switch { return &Switch{} }

func (s *Switch) Current() Services {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.services
}

func (s *Switch) Set(services Services) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = services
}
