package main

import (
	"fmt"
	"strings"

	"pedro/media"
)

// pastedImageMarker starts the note line holding a pasted image's file path.
const pastedImageMarker = "[Image file: "

// pastedImagePaths returns the files annotatePastedImages saved for a message.
func pastedImagePaths(content string) []string {
	var paths []string
	for _, line := range strings.Split(content, "\n") {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), pastedImageMarker); ok {
			if path = strings.TrimSuffix(path, "]"); path != "" {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// annotatePastedImages saves inline (pasted) images to disk and appends their
// file paths to the message, so the model can hand them to generate_image for
// editing. Images attached by path already carry one. The notes use the
// "[File: ...]" form the frontend hides, and avoid "[Path: ...]" so the image
// is not loaded a second time for vision. No-op when image generation is off.
func (a *App) annotatePastedImages(content string, imageDataURLs []string) string {
	if len(imageDataURLs) == 0 || a.pasted == nil || a.media.Current().Images == nil {
		return content
	}
	var notes []string
	for i, dataURL := range imageDataURLs {
		img, err := media.ImageFromDataURL(dataURL)
		if err != nil {
			continue
		}
		stored, err := a.pasted.Save(img)
		if err != nil {
			fmt.Println("Warning: failed to save pasted image:", err.Error())
			continue
		}
		notes = append(notes, fmt.Sprintf(
			"[File: pasted image %d]\n"+pastedImageMarker+"%s]\n[Image pasted into the chat; its pixels are included for vision. To edit it, pass this path to generate_image.]",
			i+1, stored.Path))
	}
	if len(notes) == 0 {
		return content
	}
	return content + "\n\n" + strings.Join(notes, "\n\n")
}
