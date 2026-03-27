// Package documentparser provides Go-native extraction of text from common document formats.
package documentparser

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Result is the outcome of parsing a file into line-oriented text for LLM tools.
type Result struct {
	// Format identifies the parser used (e.g. "pdf", "excel", "docx").
	Format string
	// FileSizeMB is the source file size in megabytes.
	FileSizeMB float64
	// Pages is set for PDFs when known.
	Pages int
	// Lines is 1 logical line per slice element (no leading "N: " prefixes).
	Lines []string
	// Meta holds human-readable extras (e.g. sheet list for Excel).
	Meta []string
}

// Parser extracts text lines from local files without external processes.
type Parser interface {
	Parse(path string) (*Result, error)
}

// Native implements Parser using pure Go libraries and zip/XML helpers.
type Native struct{}

// Parse routes by file extension and extracts text lines. Used by parse_document and by
// read_file for PDF only; read_file uses separate Excel/text paths (see tools/readfile.go).
func (Native) Parse(path string) (*Result, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractPDF(path)
	case ".xlsx", ".xls", ".xlsm":
		return ExtractExcelAllSheets(path)
	case ".docx":
		return extractDocx(path)
	case ".pptx":
		return extractPptx(path)
	case ".odt":
		return extractOdt(path)
	case ".html", ".htm":
		return extractHTML(path)
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".tif", ".svg", ".ico":
		return nil, ErrUnsupported{Ext: ext, Msg: "image files are not converted to text in-app (OCR not enabled)"}
	case ".doc", ".ppt", ".xlsb":
		return nil, ErrUnsupported{Ext: ext, Msg: "legacy binary Office format — export to docx/xlsx/pptx or PDF first"}
	case ".txt", ".md", ".csv", ".tsv", ".json", ".xml", ".yaml", ".yml", ".go", ".rs", ".ts", ".tsx", ".js", ".jsx", ".css", ".sql", ".sh", ".ps1", ".env", ".gitignore", ".log":
		return extractPlainText(path)
	default:
		// Unknown extension: try plain-text scan (may fail for binary).
		return extractPlainText(path)
	}
}

// ErrUnsupported indicates the format cannot be parsed in-process.
type ErrUnsupported struct {
	Ext string
	Msg string
}

func (e ErrUnsupported) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("unsupported document type %q: %s", e.Ext, e.Msg)
	}
	return fmt.Sprintf("unsupported document type %q", e.Ext)
}
