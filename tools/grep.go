package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// skipDirs contains directory names that are never searched recursively.
// Uses DefaultIgnoredDirs from default_ignore.go

type GrepTool struct{}

func NewGrepTool() *GrepTool { return &GrepTool{} }

func (GrepTool) Definition() Definition {
	return Definition{
		Name:         "grep",
		Description:  "Search for text patterns in files using regex. Returns matching lines with file:line:content format.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern to search for",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path to search in",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "Optional glob pattern to filter files by name (e.g. '*.go', '*.{ts,tsx}'). Brace expansion is supported.",
				},
				"case_insensitive": map[string]any{
					"type":        "boolean",
					"description": "Case insensitive search (default: false)",
				},
				"max_matches": map[string]any{
					"type":        "integer",
					"description": "Maximum number of matches to return (default: 200)",
				},
			},
			"required": []string{"pattern", "path"},
		},
	}
}

type grepMatch struct {
	file    string
	line    int
	content string
}

func (t GrepTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Pattern         string `json:"pattern"`
		Path            string `json:"path"`
		FilePattern     string `json:"file_pattern"`
		CaseInsensitive bool   `json:"case_insensitive"`
		MaxMatches      int    `json:"max_matches"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	if args.MaxMatches <= 0 {
		args.MaxMatches = 200
	}

	flags := ""
	if args.CaseInsensitive {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + args.Pattern)
	if err != nil {
		return fmt.Sprintf("Invalid regex pattern: %v", err), nil
	}

	// Pre-expand brace patterns in file_pattern once
	filePatterns := expandBraces(args.FilePattern)

	var matches []grepMatch

	info, err := os.Stat(args.Path)
	if err != nil {
		return fmt.Sprintf("Error accessing path: %v", err), nil
	}

	if info.IsDir() {
		err := filepath.Walk(args.Path, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				if DefaultIgnoredDirs[fi.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if args.FilePattern != "" && !matchesAnyPattern(filePatterns, fi.Name()) {
				return nil
			}
			t.searchInFile(path, re, &matches, args.MaxMatches)
			return nil
		})
		if err != nil {
			return fmt.Sprintf("Error walking directory: %v", err), nil
		}
	} else {
		t.searchInFile(args.Path, re, &matches, args.MaxMatches)
	}

	if len(matches) == 0 {
		return "No matches found.", nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].file != matches[j].file {
			return matches[i].file < matches[j].file
		}
		return matches[i].line < matches[j].line
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matches:\n", len(matches)))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", m.file, m.line, m.content))
	}
	if len(matches) == args.MaxMatches {
		sb.WriteString(fmt.Sprintf("\n(Results truncated at %d matches. Use a more specific pattern or path.)\n", args.MaxMatches))
	}
	return sb.String(), nil
}

// searchInFile scans a single file and appends regex matches to the slice.
// Stops once the global limit across all files has been reached.
func (t GrepTool) searchInFile(path string, re *regexp.Regexp, matches *[]grepMatch, limit int) {
	if len(*matches) >= limit {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer to 1 MB to handle long lines gracefully.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 1
	for scanner.Scan() {
		if len(*matches) >= limit {
			break
		}
		text := scanner.Text()
		if re.MatchString(text) {
			*matches = append(*matches, grepMatch{
				file:    path,
				line:    lineNum,
				content: strings.TrimSpace(text),
			})
		}
		lineNum++
	}
}

// expandBraces expands a simple {a,b,c} brace expression in a glob pattern.
// For example "*.{ts,tsx}" becomes ["*.ts", "*.tsx"].
// Only the first brace group is expanded; nested braces are not supported.
func expandBraces(pattern string) []string {
	if pattern == "" {
		return nil
	}
	start := strings.Index(pattern, "{")
	end := strings.Index(pattern, "}")
	if start == -1 || end == -1 || end < start {
		return []string{pattern}
	}
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")
	result := make([]string, 0, len(options))
	for _, opt := range options {
		result = append(result, prefix+strings.TrimSpace(opt)+suffix)
	}
	return result
}

// matchesAnyPattern returns true if name matches at least one of the patterns.
func matchesAnyPattern(patterns []string, name string) bool {
	for _, p := range patterns {
		if matched, err := filepath.Match(p, name); err == nil && matched {
			return true
		}
	}
	return false
}
