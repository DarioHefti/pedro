package tools

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type GlobTool struct{}

func NewGlobTool() *GlobTool { return &GlobTool{} }

func (GlobTool) Definition() Definition {
	return Definition{
		Name:         "glob",
		Description:  "Find files by name pattern using glob matching. Supports wildcard patterns like *.go, src/*.ts, and recursive **/*.go patterns.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files (e.g. '*.go', 'src/**/*.ts', '**/test*.js')",
				},
				"base_dir": map[string]any{
					"type":        "string",
					"description": "Base directory to search from (defaults to current working directory)",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

func (t GlobTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
		BaseDir string `json:"base_dir"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	baseDir := args.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	matches, err := globPattern(baseDir, args.Pattern)
	if err != nil {
		return fmt.Sprintf("Glob error: %v", err), nil
	}

	sort.Strings(matches)

	if len(matches) == 0 {
		return "No files matched the pattern.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matches:\n", len(matches)))
	for _, m := range matches {
		sb.WriteString(m + "\n")
	}
	return sb.String(), nil
}

// globPattern resolves a pattern against baseDir. When the pattern contains
// "**" it falls back to a filepath.WalkDir-based recursive search, because
// Go's stdlib filepath.Glob does not support the "**" meta-glob.
func globPattern(baseDir, pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(filepath.Join(baseDir, pattern))
	}

	// Split at the first "**" occurrence.
	// e.g. "src/**/*.go"  →  prefix="src/", suffix="*.go"
	// e.g. "**/*.go"      →  prefix="",     suffix="*.go"
	// e.g. "**"           →  prefix="",     suffix=""
	parts := strings.SplitN(pattern, "**/", 2)
	prefix := parts[0]
	suffix := ""
	if len(parts) == 2 {
		suffix = parts[1]
	}

	startDir := filepath.Join(baseDir, filepath.Clean("."+string(filepath.Separator)+prefix))
	if prefix == "" {
		startDir = baseDir
	}

	var matches []string
	err := filepath.WalkDir(startDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if DefaultIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if suffix == "" {
			matches = append(matches, path)
			return nil
		}
		// Match the suffix pattern against the file name.
		// Also try matching against the relative sub-path so that patterns
		// like "**/*.test.go" work as expected.
		if matched, _ := filepath.Match(suffix, d.Name()); matched {
			matches = append(matches, path)
			return nil
		}
		// Fallback: match suffix against path relative to startDir.
		if rel, err2 := filepath.Rel(startDir, path); err2 == nil {
			if matched, _ := filepath.Match(suffix, rel); matched {
				matches = append(matches, path)
			}
		}
		return nil
	})
	return matches, err
}
