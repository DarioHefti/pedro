package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"pedro/documentparser"
)

// ParseDocumentTool extracts text from Office/PDF/HTML and other structured documents using Go-native parsers.
type ParseDocumentTool struct{}

func NewParseDocumentTool() *ParseDocumentTool { return &ParseDocumentTool{} }

func (ParseDocumentTool) Definition() Definition {
	return Definition{
		Name:         "parse_document",
		Description:  "Extract human-readable text from a local document (PDF, Excel, Word DOCX, PowerPoint PPTX, ODT, HTML, or plain text). Prefer this over read_file for PDFs and Office files. Output is paginated like read_file (50 KB max per call). For Excel, omit 'sheet' to include all sheets, or set 'sheet' to a specific sheet name.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the file",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line number to start from, 1-indexed (default: 1)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to return (default: 2000)",
				},
				"sheet": map[string]any{
					"type":        "string",
					"description": "For Excel only: sheet name to extract (omit to parse all sheets)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (ParseDocumentTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
		Sheet  string `json:"sheet"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	args.Path = filepath.Clean(strings.TrimSpace(args.Path))
	if !filepath.IsAbs(args.Path) {
		return "Document parse error: path must be absolute", nil
	}
	if args.Offset < 1 {
		args.Offset = 1
	}
	if args.Limit <= 0 {
		args.Limit = readDefaultLimit
	}

	ext := strings.ToLower(filepath.Ext(args.Path))
	var res *documentparser.Result
	var err error

	switch ext {
	case ".xlsx", ".xls", ".xlsm":
		if strings.TrimSpace(args.Sheet) != "" {
			res, err = documentparser.ExtractExcelSheet(args.Path, args.Sheet)
		} else {
			res, err = documentparser.ExtractExcelAllSheets(args.Path)
		}
	default:
		if args.Sheet != "" {
			return fmt.Sprintf("The 'sheet' parameter only applies to Excel files. Path has extension %q.", ext), nil
		}
		res, err = (documentparser.Native{}).Parse(args.Path)
	}

	if err != nil {
		return fmt.Sprintf("Document parse error: %v", err), nil
	}

	return formatParseDocumentOutput(args.Path, res, args.Offset, args.Limit), nil
}

func formatParseDocumentOutput(path string, res *documentparser.Result, offset, limit int) string {
	if res == nil {
		return "(empty parse result)"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Parser: %s | File: %s (%.1f MB)", res.Format, path, res.FileSizeMB)
	if res.Pages > 0 {
		fmt.Fprintf(&sb, " | %d pages", res.Pages)
	}
	sb.WriteString("\n")
	if len(res.Meta) > 0 {
		sb.WriteString("Meta:\n")
		for _, m := range res.Meta {
			sb.WriteString("  - ")
			sb.WriteString(m)
			sb.WriteByte('\n')
		}
		sb.WriteString("\n")
	}

	lines := res.Lines
	totalLines := len(lines)
	if totalLines == 0 {
		sb.WriteString("(No lines extracted.)")
		return sb.String()
	}

	if offset > totalLines {
		fmt.Fprintf(&sb, "(No content at offset %d — document has %d lines total)", offset, totalLines)
		return sb.String()
	}

	end := offset + limit - 1
	if end > totalLines {
		end = totalLines
	}
	hasMore := end < totalLines

	bytesUsed := 0
	truncatedByBytes := false
	lastLine := offset - 1

	for i := offset - 1; i < end; i++ {
		text := lines[i]
		if len(text) > readMaxLineLen {
			text = fmt.Sprintf("%s... (%d chars, truncated)", text[:readMaxLineLen], len(text))
		}
		entry := fmt.Sprintf("%d: %s", i+1, text)
		if bytesUsed+len(entry)+1 > readMaxBytes && i > offset-1 {
			truncatedByBytes = true
			hasMore = true
			break
		}
		sb.WriteString(entry)
		sb.WriteByte('\n')
		bytesUsed += len(entry) + 1
		lastLine = i + 1
	}

	nextOffset := lastLine + 1
	switch {
	case truncatedByBytes:
		fmt.Fprintf(&sb, "\n(Output capped at 50 KB. Showing lines %d-%d. Call parse_document with offset=%d to continue.)",
			offset, lastLine, nextOffset)
	case hasMore:
		fmt.Fprintf(&sb, "\n(Showing lines %d-%d of %d. Call parse_document with offset=%d to continue.)",
			offset, lastLine, totalLines, nextOffset)
	default:
		fmt.Fprintf(&sb, "\n(End of document — %d lines total)", totalLines)
	}

	return sb.String()
}
