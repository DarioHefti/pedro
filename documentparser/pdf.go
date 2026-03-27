package documentparser

import (
	"fmt"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

func extractPDF(path string) (*Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	pdfReader, err := pdf.NewReader(f, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}

	numPages := pdfReader.NumPage()
	var lines []string

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("--- Page %d ---", i))
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				lines = append(lines, trimmed)
			}
		}
	}

	return &Result{
		Format:     "pdf",
		FileSizeMB: fileSizeMB,
		Pages:      numPages,
		Lines:      lines,
	}, nil
}
