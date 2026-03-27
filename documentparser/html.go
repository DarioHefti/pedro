package documentparser

import (
	"os"
	"strings"

	htmlmd "github.com/JohannesKaufmann/html-to-markdown"
)

func extractHTML(path string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	conv := htmlmd.NewConverter("", true, nil)
	conv.Remove("script", "style", "noscript", "nav", "header",
		"footer", "aside", "iframe", "svg", "canvas")
	md, err := conv.ConvertString(string(b))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		out = []string{"(No text content extracted from HTML)"}
	}
	return &Result{
		Format:     "html",
		FileSizeMB: fileSizeMB,
		Lines:      out,
	}, nil
}
