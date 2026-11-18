package documentparser

import (
	"fmt"
	"math"
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
		text, err := pageText(page)
		if err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("--- Page %d ---", i))
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.Join(strings.Fields(line), " ")
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

// pageText rebuilds a page's text from its glyph positions. Many PDFs (e.g.
// from LaTeX) position words instead of writing space characters, which makes
// plain text extraction run all words together.
func pageText(page pdf.Page) (text string, err error) {
	defer func() {
		if r := recover(); r != nil { // the pdf package panics on some malformed files
			err = fmt.Errorf("malformed page: %v", r)
		}
	}()
	if glyphs := page.Content().Text; len(glyphs) > 0 {
		return joinGlyphs(glyphs), nil
	}
	return page.GetPlainText(nil)
}

// joinGlyphs joins glyphs in content order, starting a new line when the
// baseline moves and inserting a space where the horizontal gap to the
// previous glyph is wider than kerning.
func joinGlyphs(glyphs []pdf.Text) string {
	var sb strings.Builder
	for i, g := range glyphs {
		if i > 0 {
			prev := glyphs[i-1]
			size := max(prev.FontSize, g.FontSize, 1)
			gap := g.X - (prev.X + prev.W)
			switch {
			case math.Abs(g.Y-prev.Y) > size/2:
				sb.WriteByte('\n')
			case gap > size*0.15 || gap < -size:
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(g.S)
	}
	return sb.String()
}
