package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"pedro/shared"
)

// MemorySaveTool stores a fact about the user to long-term memory.
type MemorySaveTool struct {
	backend shared.MemoryBackend
}

func NewMemorySaveTool(backend shared.MemoryBackend) *MemorySaveTool {
	return &MemorySaveTool{backend: backend}
}

func (t *MemorySaveTool) Definition() Definition {
	return Definition{
		Name: "memory_save",
		Description: "Save an important fact about the user to long-term memory. Call this whenever you learn something about the user that would be useful to remember in future conversations (name, preferences, job, tech stack, goals, etc.)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":      map[string]any{"type": "string", "description": "Short semantic key (e.g., 'user_name', 'preferred_language', 'tech_stack')"},
				"value":    map[string]any{"type": "string", "description": "The value to remember"},
				"category": map[string]any{"type": "string", "description": "Optional category: personal, technical, preference, goal, other"},
			},
			"required": []string{"key", "value"},
		},
	}
}

func (t *MemorySaveTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Key == "" || args.Value == "" {
		return "", fmt.Errorf("key and value are required")
	}
	category := args.Category
	if category == "" {
		category = "general"
	}
	if err := t.backend.SaveMemory(args.Key, args.Value, category); err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}
	return fmt.Sprintf("Memory saved: %s = %s", args.Key, args.Value), nil
}

// MemorySearchTool searches the user's long-term memory.
type MemorySearchTool struct {
	backend shared.MemoryBackend
}

func NewMemorySearchTool(backend shared.MemoryBackend) *MemorySearchTool {
	return &MemorySearchTool{backend: backend}
}

func (t *MemorySearchTool) Definition() Definition {
	return Definition{
		Name:        "memory_search",
		Description: "Retrieve one or more memories by key, or search by keywords. Prefer 'keys' for exact lookup when you see matching keys in Available Memory Keys.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keys":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of exact memory keys to retrieve (e.g., [\"address\", \"name\", \"city\"])"},
				"query": map[string]any{"type": "string", "description": "Search query (keywords to match against memory keys and values)"},
			},
		},
		DeferLoading: true,
	}
}

func (t *MemorySearchTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Keys  []string `json:"keys"`
		Query string   `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if len(args.Keys) > 0 {
		var results []string
		for _, key := range args.Keys {
			records, err := t.backend.SearchMemories(key)
			if err != nil {
				return "", fmt.Errorf("search failed: %w", err)
			}
			found := false
			for _, r := range records {
				if r.Key == key {
					results = append(results, fmt.Sprintf("%s (%s): %s", r.Key, r.Category, r.Value))
					found = true
					break
				}
			}
			if !found {
				results = append(results, fmt.Sprintf("%s: not found", key))
			}
		}
		return strings.Join(results, "\n"), nil
	}
	records, err := t.backend.SearchMemories(args.Query)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}
	if len(records) == 0 {
		return "No memories found matching that query.", nil
	}
	var b strings.Builder
	b.WriteString("Memories found:\n")
	for _, r := range records {
		b.WriteString(fmt.Sprintf("- [%d] %s (%s): %s\n", r.ID, r.Key, r.Category, r.Value))
	}
	return b.String(), nil
}

// MemoryForgetTool deletes a specific memory by ID.
type MemoryForgetTool struct {
	backend shared.MemoryBackend
}

func NewMemoryForgetTool(backend shared.MemoryBackend) *MemoryForgetTool {
	return &MemoryForgetTool{backend: backend}
}

func (t *MemoryForgetTool) Definition() Definition {
	return Definition{
		Name:        "memory_forget",
		Description: "Delete a specific memory by its ID. Call this when a memory is outdated, incorrect, or the user asks you to forget something.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "integer", "description": "The memory ID to delete"},
			},
			"required": []string{"id"},
		},
		DeferLoading: true,
	}
}

func (t *MemoryForgetTool) Execute(argsJSON string) (string, error) {
	var args struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if err := t.backend.ForgetMemory(args.ID); err != nil {
		return "", fmt.Errorf("failed to forget memory: %w", err)
	}
	return fmt.Sprintf("Memory %d deleted.", args.ID), nil
}
