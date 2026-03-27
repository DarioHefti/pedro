package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"pedro/documentparser"
)

const (
	readDefaultLimit = 2000
	readMaxBytes     = 50 * 1024
	readMaxLineLen   = 2000
)

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool { return &ReadFileTool{} }

func (ReadFileTool) Definition() Definition {
	return Definition{
		Name:        "read_file",
		Description: "Read a local file in paginated chunks. Always use offset and limit for large files. The response tells you the next offset to use if there is more content. For Excel files, it shows sheet metadata and reads data as CSV format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the file",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line/row number to start reading from, 1-indexed (default: 1)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines/rows to read (default: 2000)",
				},
				"sheet": map[string]any{
					"type":        "string",
					"description": "For Excel files: sheet name to read (default: first sheet)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (r ReadFileTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
		Sheet  string `json:"sheet"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	result, err := r.read(args.Path, args.Offset, args.Limit, args.Sheet)
	if err != nil {
		return fmt.Sprintf("File read error: %v", err), nil
	}
	return result, nil
}

// read routes the read_file tool by extension. PDF uses documentparser (same extraction as
// parse_document). Excel uses readExcel (sheet-scoped rows + read_file-specific formatting).
// Other extensions use line-oriented text; for docx/pptx/etc. the model should prefer parse_document.
func (ReadFileTool) read(path string, offset, limit int, sheet string) (string, error) {
	if offset < 1 {
		offset = 1
	}
	if limit <= 0 {
		limit = readDefaultLimit
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return readPDF(path, offset, limit)
	case ".xlsx", ".xls", ".xlsm":
		return readExcel(path, offset, limit, sheet)
	default:
		return readTextFile(path, offset, limit)
	}
}

func readTextFile(path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var lines []string
	lineNum := 0
	bytesUsed := 0
	truncatedByBytes := false
	hasMore := false

	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if len(lines) >= limit {
			hasMore = true
			break
		}

		text := scanner.Text()
		if len(text) > readMaxLineLen {
			text = fmt.Sprintf("%s... (%d chars, truncated)", text[:readMaxLineLen], len(text))
		}

		entry := fmt.Sprintf("%d: %s", lineNum, text)
		if bytesUsed+len(entry)+1 > readMaxBytes && len(lines) > 0 {
			truncatedByBytes = true
			hasMore = true
			break
		}
		lines = append(lines, entry)
		bytesUsed += len(entry) + 1
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "File: %s (%.1f MB)\n\n", path, fileSizeMB)

	if len(lines) == 0 {
		sb.WriteString("(No content at this offset — file may be empty or offset is past end of file)")
		return sb.String(), nil
	}

	for _, l := range lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}

	lastLine := offset + len(lines) - 1
	nextOffset := lastLine + 1

	switch {
	case truncatedByBytes:
		fmt.Fprintf(&sb, "\n(Output capped at 50 KB. Showing lines %d-%d. Call read_file with offset=%d to continue.)",
			offset, lastLine, nextOffset)
	case hasMore:
		fmt.Fprintf(&sb, "\n(Showing lines %d-%d. Call read_file with offset=%d to continue.)",
			offset, lastLine, nextOffset)
	default:
		fmt.Fprintf(&sb, "\n(End of file — %d lines total)", lastLine)
	}

	return sb.String(), nil
}

func readExcel(path string, offset, limit int, sheetName string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to open Excel file: %v", err)
	}
	defer f.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	// Get all sheet names and their row counts for metadata
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return "", fmt.Errorf("Excel file has no sheets")
	}

	// Build sheet metadata
	type sheetInfo struct {
		name     string
		rowCount int
	}
	var sheets []sheetInfo
	for _, name := range sheetList {
		rows, err := f.GetRows(name)
		if err != nil {
			sheets = append(sheets, sheetInfo{name: name, rowCount: 0})
		} else {
			sheets = append(sheets, sheetInfo{name: name, rowCount: len(rows)})
		}
	}

	// Select sheet to read
	targetSheet := sheetName
	if targetSheet == "" {
		targetSheet = sheetList[0]
	}

	// Verify sheet exists
	sheetExists := false
	for _, name := range sheetList {
		if name == targetSheet {
			sheetExists = true
			break
		}
	}
	if !sheetExists {
		var sb strings.Builder
		fmt.Fprintf(&sb, "File: %s (%.1f MB)\n", path, fileSizeMB)
		sb.WriteString("Available sheets:\n")
		for _, s := range sheets {
			fmt.Fprintf(&sb, "  - %s (%d rows)\n", s.name, s.rowCount)
		}
		fmt.Fprintf(&sb, "\nError: Sheet '%s' not found", sheetName)
		return sb.String(), nil
	}

	// Get rows from target sheet
	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return "", fmt.Errorf("failed to read sheet '%s': %v", targetSheet, err)
	}

	totalRows := len(rows)
	var sb strings.Builder

	// Header with metadata
	fmt.Fprintf(&sb, "File: %s (%.1f MB)\n", path, fileSizeMB)
	sb.WriteString("Sheets:\n")
	for _, s := range sheets {
		marker := "  "
		if s.name == targetSheet {
			marker = "→ "
		}
		fmt.Fprintf(&sb, "%s%s (%d rows)\n", marker, s.name, s.rowCount)
	}
	sb.WriteString("\n")

	if totalRows == 0 {
		fmt.Fprintf(&sb, "Sheet '%s' is empty", targetSheet)
		return sb.String(), nil
	}

	if offset > totalRows {
		fmt.Fprintf(&sb, "(No content at offset %d — sheet has %d rows total)", offset, totalRows)
		return sb.String(), nil
	}

	// Read rows with pagination
	end := offset + limit - 1
	if end > totalRows {
		end = totalRows
	}
	hasMore := end < totalRows

	bytesUsed := 0
	truncatedByBytes := false
	lastRow := offset - 1

	for i := offset - 1; i < end; i++ {
		row := rows[i]
		csvLine := documentparser.FormatRowAsCSV(row)
		entry := fmt.Sprintf("%d: %s", i+1, csvLine)

		if len(entry) > readMaxLineLen {
			entry = fmt.Sprintf("%d: %s... (truncated)", i+1, entry[:readMaxLineLen])
		}

		if bytesUsed+len(entry)+1 > readMaxBytes && i > offset-1 {
			truncatedByBytes = true
			hasMore = true
			break
		}

		sb.WriteString(entry)
		sb.WriteByte('\n')
		bytesUsed += len(entry) + 1
		lastRow = i + 1
	}

	nextOffset := lastRow + 1

	switch {
	case truncatedByBytes:
		fmt.Fprintf(&sb, "\n(Output capped at 50 KB. Showing rows %d-%d of %d. Call read_file with offset=%d to continue.)",
			offset, lastRow, totalRows, nextOffset)
	case hasMore:
		fmt.Fprintf(&sb, "\n(Showing rows %d-%d of %d. Call read_file with offset=%d to continue.)",
			offset, lastRow, totalRows, nextOffset)
	default:
		fmt.Fprintf(&sb, "\n(End of sheet — %d rows total)", totalRows)
	}

	return sb.String(), nil
}

func readPDF(path string, offset, limit int) (string, error) {
	res, err := (documentparser.Native{}).Parse(path)
	if err != nil {
		return "", err
	}
	allLines := res.Lines
	fileSizeMB := res.FileSizeMB
	numPages := res.Pages

	totalLines := len(allLines)
	if offset > totalLines {
		var sb strings.Builder
		fmt.Fprintf(&sb, "File: %s (%.1f MB)\n\n", path, fileSizeMB)
		sb.WriteString(fmt.Sprintf("(No content at offset %d — file has %d lines total)", offset, totalLines))
		return sb.String(), nil
	}

	end := offset + limit - 1
	if end > totalLines {
		end = totalLines
	}
	hasMore := end < totalLines

	var sb strings.Builder
	fmt.Fprintf(&sb, "File: %s (%.1f MB, %d pages)\n\n", path, fileSizeMB, numPages)

	if totalLines == 0 {
		sb.WriteString("(No text content found in PDF)")
		return sb.String(), nil
	}

	for i := offset - 1; i < end; i++ {
		sb.WriteString(fmt.Sprintf("%d: %s\n", i+1, allLines[i]))
	}

	nextOffset := end + 1
	if hasMore {
		fmt.Fprintf(&sb, "\n(Showing lines %d-%d of %d. Call read_file with offset=%d to continue.)",
			offset, end, totalLines, nextOffset)
	} else {
		fmt.Fprintf(&sb, "\n(End of file — %d lines total)", totalLines)
	}

	return sb.String(), nil
}
