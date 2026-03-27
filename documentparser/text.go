package documentparser

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func extractPlainText(path string) (*Result, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) && bytes.Contains(data, []byte{0}) {
		return nil, ErrUnsupported{Ext: filepath.Ext(path), Msg: "file appears binary; use a supported Office/PDF format or plain text"}
	}

	var lines []string
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, line)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return &Result{
		Format:     "text",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
	}, nil
}
