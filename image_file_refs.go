package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// maxImageFileBytes caps a single on-disk image read for vision (file-ref attachments).
const maxImageFileBytes = 25 << 20 // 25 MiB

// mergeImageDataURLsFromFileRefs appends data:image/...;base64,... URLs for raster images
// referenced either via file-ref attachments or [Path: ...] markers in message content.
func mergeImageDataURLsFromFileRefs(imageDataURLs []string, attachmentsJSON, content string) []string {
	out := append([]string(nil), imageDataURLs...)
	candidates := make([]string, 0, 8)

	var atts []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if attachmentsJSON != "" {
		if err := json.Unmarshal([]byte(attachmentsJSON), &atts); err == nil {
			for _, a := range atts {
				if a.Type == "file-ref" {
					candidates = append(candidates, strings.TrimSpace(a.Content))
				}
			}
		}
	}

	// Fallback for cases where the path is in user content but not in structured attachments.
	for _, p := range extractBracketPathMarkers(content) {
		candidates = append(candidates, p)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		dataURL, err := filePathToImageDataURL(p)
		if err != nil {
			continue
		}
		out = append(out, dataURL)
	}
	return out
}

func extractBracketPathMarkers(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	paths := make([]string, 0, 4)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, "]") {
			continue
		}
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "[path:") {
			continue
		}
		v := strings.TrimSuffix(line, "]")
		v = strings.TrimSpace(v[6:]) // len("[Path:")
		v = trimWrappedQuotes(v)
		v = strings.TrimSpace(v)
		if v != "" {
			paths = append(paths, v)
		}
	}
	return paths
}

func trimWrappedQuotes(v string) string {
	if len(v) < 2 {
		return v
	}
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		return strings.Trim(v, "\"")
	}
	if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
		return strings.Trim(v, "'")
	}
	return v
}

func filePathToImageDataURL(path string) (string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path not absolute")
	}
	if !isRasterImageExt(path) {
		return "", fmt.Errorf("not a raster image extension")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxImageFileBytes {
		return "", fmt.Errorf("image too large")
	}
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" || !strings.HasPrefix(mimeType, "image/") {
		mimeType = "application/octet-stream"
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".bmp":
			mimeType = "image/bmp"
		case ".tif", ".tiff":
			mimeType = "image/tiff"
		default:
			return "", fmt.Errorf("unknown image media type")
		}
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64), nil
}

func isRasterImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
