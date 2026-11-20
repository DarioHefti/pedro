package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"pedro/media"
)

type stubImages struct{}

func (stubImages) GenerateImage(context.Context, media.ImageRequest) (media.Image, error) {
	return media.Image{}, nil
}

func TestAnnotatePastedImages(t *testing.T) {
	a := &App{media: media.NewSwitch(), pasted: media.NewDirImageStore(t.TempDir(), media.PastedImagesRoute)}
	pasted := []string{"data:image/png;base64,aGk=", "not-a-data-url"}

	if got := a.annotatePastedImages("hello", pasted); got != "hello" {
		t.Fatalf("image generation off: content changed to %q", got)
	}

	a.media.Set(media.Services{Images: stubImages{}})
	got := a.annotatePastedImages("hello", pasted)
	m := regexp.MustCompile(`\[Image file: (.+)\]`).FindStringSubmatch(got)
	if !strings.HasPrefix(got, "hello\n\n[File: pasted image 1]\n") || m == nil {
		t.Fatalf("unexpected annotation:\n%s", got)
	}
	if data, err := os.ReadFile(m[1]); err != nil || string(data) != "hi" {
		t.Fatalf("pasted image not saved: %v", err)
	}
	if strings.Count(got, "[File:") != 1 {
		t.Fatalf("invalid data URL should be skipped:\n%s", got)
	}
	// Must not use the [Path: ...] marker, which would load the image twice for vision.
	if len(extractBracketPathMarkers(got)) != 0 {
		t.Fatal("annotation must not contain [Path: ...] markers")
	}
}

// fakeStore implements the parts of Store that chat deletion uses.
type fakeStore struct {
	Store
	messages map[int64][]Message
}

func (f *fakeStore) GetMessages(id int64) ([]Message, error) { return f.messages[id], nil }
func (f *fakeStore) DeleteConversation(id int64) error {
	delete(f.messages, id)
	return nil
}
func (f *fakeStore) DeleteAllConversations() error {
	f.messages = map[int64][]Message{}
	return nil
}

func TestDeleteConversationRemovesItsImages(t *testing.T) {
	images := media.NewDirImageStore(t.TempDir(), media.GeneratedImagesRoute)
	pasted := media.NewDirImageStore(t.TempDir(), media.PastedImagesRoute)
	a := &App{media: media.NewSwitch(), images: images, pasted: pasted}
	a.media.Set(media.Services{Images: stubImages{}})

	save := func(s media.ImageStore) string {
		stored, err := s.Save(media.Image{Data: []byte("x"), Format: "png"})
		if err != nil {
			t.Fatal(err)
		}
		return stored.Path
	}
	otherChatImage := save(images)
	generated := save(images)
	userContent := a.annotatePastedImages("edit this", []string{"data:image/png;base64,aGk="})
	pastedPath := pastedImagePaths(userContent)[0]

	a.store = &fakeStore{messages: map[int64][]Message{
		1: {
			{Role: "user", Content: userContent},
			// The chat edited an image from another chat: referenced, not created here.
			{Role: "assistant", ToolCalls: `[{"name":"generate_image","argsJSON":"{\"images\":[\"` + otherChatImage + `\"]}"}]`},
			{Role: "tool", Content: "Image edited and saved to: " + generated + "\n\n![x](/generated-images/x.png)"},
			{Role: "assistant", Content: "![x](" + media.GeneratedImagesRoute + filepath.Base(otherChatImage) + ")"},
		},
	}}

	if err := a.DeleteConversation(1); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{generated, pastedPath} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s should have been deleted", gone)
		}
	}
	if _, err := os.Stat(otherChatImage); err != nil {
		t.Error("image of another chat must be kept")
	}

	if err := a.DeleteAllConversations(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(otherChatImage); !os.IsNotExist(err) {
		t.Error("deleting all chats should remove all images")
	}
}
