package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
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
		Description: "Read a local file in paginated chunks. Always use offset and limit for large files. The response tells you the next offset to use if there is more content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the file",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line number to start reading from, 1-indexed (default: 1)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to read (default: 2000)",
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
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	result, err := r.read(args.Path, args.Offset, args.Limit)
	if err != nil {
		return fmt.Sprintf("File read error: %v", err), nil
	}
	return result, nil
}

func (ReadFileTool) read(path string, offset, limit int) (string, error) {
	if offset < 1 {
		offset = 1
	}
	if limit <= 0 {
		limit = readDefaultLimit
	}

	ext := strings.ToLower(path)
	if strings.HasSuffix(ext, ".pdf") {
		return readPDF(path, offset, limit)
	}

	return readTextFile(path, offset, limit)
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

func readPDF(path string, offset, limit int) (string, error) {
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

	pdfReader, err := pdf.NewReader(f, stat.Size())
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %v", err)
	}

	numPages := pdfReader.NumPage()
	var allLines []string

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		allLines = append(allLines, fmt.Sprintf("--- Page %d ---", i))
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				allLines = append(allLines, trimmed)
			}
		}
	}

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
