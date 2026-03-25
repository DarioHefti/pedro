package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	showTreeDefaultDepth = 4
	showTreeMaxDepth     = 32
	showTreeDefaultLimit = 500
	showTreeMaxLimit     = 500
)

// ShowFileTreeTool lists a directory tree up to a configurable depth so the model
// can locate files without guessing paths.
type ShowFileTreeTool struct{}

func NewShowFileTreeTool() *ShowFileTreeTool { return &ShowFileTreeTool{} }

func (ShowFileTreeTool) Definition() Definition {
	return Definition{
		Name:        "show_file_tree",
		Description: "List files and subfolders under a local directory up to a given depth. Output is paginated (default 500 tree lines per call). If the response says there is more, call again with the same path and depth and the given offset to continue. Use when the user gives a folder path and you need to find a file by name or structure.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to the directory to list",
				},
				"depth": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("How many directory levels to expand below the given path (1 = only immediate children, no recursion into subfolders). Default %d, maximum %d.", showTreeDefaultDepth, showTreeMaxDepth),
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-based index of the first tree line to return (same ordering as previous calls). Default 1. After a truncated page, set this to the next_offset value from the footer.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Maximum number of tree lines to return this call (default %d, maximum %d).", showTreeDefaultLimit, showTreeMaxLimit),
				},
			},
			"required": []string{"path"},
		},
	}
}

func (ShowFileTreeTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Path   string `json:"path"`
		Depth  int    `json:"depth"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}
	if args.Path == "" {
		return "Error: path is required", nil
	}
	depth := args.Depth
	if depth <= 0 {
		depth = showTreeDefaultDepth
	}
	if depth > showTreeMaxDepth {
		depth = showTreeMaxDepth
	}
	offset := args.Offset
	if offset < 1 {
		offset = 1
	}
	limit := args.Limit
	if limit <= 0 {
		limit = showTreeDefaultLimit
	}
	if limit > showTreeMaxLimit {
		limit = showTreeMaxLimit
	}
	out, err := buildFileTree(args.Path, depth, offset, limit)
	if err != nil {
		return fmt.Sprintf("File tree error: %v", err), nil
	}
	return out, nil
}

func buildFileTree(root string, maxDepth, offset, limit int) (string, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	st, err := os.Stat(absRoot)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", absRoot)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Root: %s\n", absRoot)
	fmt.Fprintf(&sb, "Depth: %d (levels expanded below root; 1 = flat list only)\n", maxDepth)
	fmt.Fprintf(&sb, "Page: offset=%d, limit=%d (1-based line index into the full tree listing)\n\n", offset, limit)

	visited := make(map[string]struct{})
	var globalLine int
	var emitted int
	var truncated bool
	var stop bool

	var walk func(dir string, prefix string, depthLeft int) error
	walk = func(dir string, prefix string, depthLeft int) error {
		if stop {
			return nil
		}

		realDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			realDir = dir
		}
		realDir, err = filepath.Abs(realDir)
		if err != nil {
			return err
		}
		if _, seen := visited[realDir]; seen {
			globalLine++
			if globalLine >= offset && emitted < limit {
				fmt.Fprintf(&sb, "%s[loop/symlink skipped: %s]\n", prefix, dir)
				emitted++
				if emitted >= limit {
					truncated = true
					stop = true
				}
			}
			return nil
		}
		visited[realDir] = struct{}{}
		defer delete(visited, realDir)

		entries, err := os.ReadDir(dir)
		if err != nil {
			globalLine++
			if globalLine >= offset && emitted < limit {
				fmt.Fprintf(&sb, "%s[error reading: %v]\n", prefix, err)
				emitted++
				if emitted >= limit {
					truncated = true
					stop = true
				}
			}
			return nil
		}

		type named struct {
			name  string
			isDir bool
		}
		var names []named
		for _, e := range entries {
			names = append(names, named{name: e.Name(), isDir: e.IsDir()})
		}
		sort.Slice(names, func(i, j int) bool {
			if names[i].isDir != names[j].isDir {
				return names[i].isDir
			}
			return strings.ToLower(names[i].name) < strings.ToLower(names[j].name)
		})

		for i, n := range names {
			if stop {
				return nil
			}
			marker := "├── "
			if i == len(names)-1 {
				marker = "└── "
			}
			childPath := filepath.Join(dir, n.name)

			globalLine++
			if globalLine >= offset && emitted < limit {
				if n.isDir {
					fmt.Fprintf(&sb, "%s%s%s/\n", prefix, marker, n.name)
				} else {
					fmt.Fprintf(&sb, "%s%s%s\n", prefix, marker, n.name)
				}
				emitted++
				if emitted >= limit {
					truncated = true
					stop = true
					return nil
				}
			}

			if !n.isDir || depthLeft <= 1 {
				continue
			}
			ext := "│   "
			if i == len(names)-1 {
				ext = "    "
			}
			if err := walk(childPath, prefix+ext, depthLeft-1); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(absRoot, "", maxDepth); err != nil {
		return "", err
	}

	totalLines := globalLine
	firstShown := offset
	lastShown := offset + emitted - 1
	if emitted > 0 {
		if truncated {
			fmt.Fprintf(&sb, "\n---\nShowing tree lines %d–%d on this page (listing continues beyond this page).\n", firstShown, lastShown)
		} else {
			fmt.Fprintf(&sb, "\n---\nShowing tree lines %d–%d of %d (complete listing).\n", firstShown, lastShown, totalLines)
		}
	} else if offset > 1 && totalLines > 0 {
		fmt.Fprintf(&sb, "\n---\nNo lines on this page (offset %d is past the end; total tree lines = %d). Use offset=1 to restart.\n", offset, totalLines)
	} else if totalLines == 0 {
		fmt.Fprintf(&sb, "(Empty directory — no entries.)\n")
	}

	switch {
	case truncated:
		next := offset + emitted
		fmt.Fprintf(&sb, "\n**Truncated:** capped at %d tree lines per request. There is more after line %d.\n", limit, lastShown)
		fmt.Fprintf(&sb, "Call **show_file_tree** again with the same `path` and `depth`, **`offset=%d`**, and the same `limit` (or omit `limit` for default %d) to continue.\n", next, showTreeDefaultLimit)
	case emitted > 0 && !truncated:
		fmt.Fprintf(&sb, "\n**End of listing** — all %d tree lines were shown; no further pagination needed.\n", totalLines)
	}

	return sb.String(), nil
}
