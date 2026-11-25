package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const ToolSearchToolName = "tool_search"

type ToolSearchTool struct {
	registry *Registry
}

func NewToolSearchTool(registry *Registry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t ToolSearchTool) Definition() Definition {
	return Definition{
		Name:        ToolSearchToolName,
		Description: "Search for tools by name or description to load them on-demand. Use this when you need a specific tool but don't have its full definition available.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query - can be a regex pattern or natural language description of the tool you need",
				},
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"regex", "bm25"},
					"description": "Search mode: regex uses Python regex patterns, bm25 uses natural language",
				},
			},
			"required": []string{"query"},
		},
		DeferLoading: false,
	}
}

type ToolReference struct {
	ToolName string `json:"tool_name"`
}

type ToolSearchResult struct {
	Type          string          `json:"type"`
	ToolReference []ToolReference `json:"tool_references"`
	// Message tells the model the tools are loaded, so it calls them instead of searching again.
	Message string `json:"message"`
}

const noToolsMatchedResult = `{"tool_references": [], "message": "No tools matched. Search with an exact tool name from the system prompt."}`

func loadedToolsMessage(refs []ToolReference) string {
	names := make([]string, len(refs))
	for i, ref := range refs {
		names[i] = ref.ToolName
	}
	return "Loaded " + strings.Join(names, ", ") + " for the rest of the conversation. Call them directly; do not search for them again."
}

func (t ToolSearchTool) Execute(argsJSON string) (string, error) {
	if t.registry == nil {
		return `{"error": "tool registry is not configured"}`, nil
	}

	var args struct {
		Query string `json:"query"`
		Mode  string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error": "Error parsing arguments: %v"}`, err), nil
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		return `{"error": "query is required"}`, nil
	}

	mode := strings.ToLower(strings.TrimSpace(args.Mode))
	if mode == "" {
		mode = "regex"
	}

	references := t.exactMatches(query)
	if len(references) == 0 {
		references = t.search(query, mode)
	}
	if len(references) == 0 && mode == "regex" {
		// Models often send a natural-language description in regex mode.
		references = t.search(query, "bm25")
	}
	if len(references) == 0 {
		return noToolsMatchedResult, nil
	}

	result := ToolSearchResult{
		Type:          "tool_reference",
		ToolReference: references,
		Message:       loadedToolsMessage(references),
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to encode result: %v"}`, err), nil
	}
	return string(data), nil
}

// exactMatches returns the named tools when the query only lists tool names
// ("web_search", "web_search|webclaw"), so a search by name loads just those
// tools and not every tool whose description mentions them.
func (t ToolSearchTool) exactMatches(query string) []ToolReference {
	searchable := map[string]bool{}
	for _, def := range t.registry.AllDefinitions() {
		if def.DeferLoading && def.Name != ToolSearchToolName {
			searchable[def.Name] = true
		}
	}
	names := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return strings.ContainsRune("|,;() \t\n^$", r)
	})
	var refs []ToolReference
	seen := map[string]bool{}
	for _, name := range names {
		if !searchable[name] {
			return nil
		}
		if !seen[name] {
			seen[name] = true
			refs = append(refs, ToolReference{ToolName: name})
		}
	}
	return refs
}

func (t ToolSearchTool) search(query, mode string) []ToolReference {
	defs := t.registry.AllDefinitions()

	type scoredDef struct {
		def   Definition
		score int
	}
	var scored []scoredDef

	for _, def := range defs {
		if def.Name == ToolSearchToolName {
			continue
		}
		if !def.DeferLoading {
			continue
		}

		score := t.calculateScore(def, query, mode)
		if score > 0 {
			scored = append(scored, scoredDef{def: def, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].def.Name < scored[j].def.Name
	})

	maxResults := 5
	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	refs := make([]ToolReference, len(scored))
	for i, s := range scored {
		refs[i].ToolName = s.def.Name
	}
	return refs
}

func (t ToolSearchTool) calculateScore(def Definition, query, mode string) int {
	name := strings.ToLower(def.Name)
	desc := strings.ToLower(def.Description)

	// Drill into the JSON Schema "properties" object to extract argument names
	// and descriptions. The top-level Parameters map contains "type", "properties",
	// and "required" — the actual per-argument data lives under "properties".
	argInfo := ""
	if props, ok := def.Parameters["properties"].(map[string]any); ok {
		for paramName, paramDef := range props {
			argInfo += " " + paramName
			if pd, ok := paramDef.(map[string]any); ok {
				if d, ok := pd["description"].(string); ok {
					argInfo += " " + d
				}
			}
		}
	}
	argInfo = strings.ToLower(argInfo)

	switch mode {
	case "bm25":
		return t.bm25Score(query, name, desc, argInfo)
	default:
		return t.regexScore(query, name, desc, argInfo)
	}
}

func (t ToolSearchTool) regexScore(pattern, name, desc, argInfo string) int {
	score := 0

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		if strings.Contains(strings.ToLower(pattern), strings.ToLower(name)) {
			return 50
		}
		return 0
	}

	if re.FindString(name) != "" {
		score += 100
	}
	if re.FindString(desc) != "" {
		score += 50
	}
	if re.FindString(argInfo) != "" {
		score += 25
	}

	return score
}

func (t ToolSearchTool) bm25Score(query, name, desc, argInfo string) int {
	score := 0

	queryLower := strings.ToLower(query)
	nameLower := name
	descLower := desc
	argLower := argInfo

	if strings.Contains(nameLower, queryLower) {
		score += 100
	} else if strings.Contains(queryLower, nameLower) {
		score += 80
	}

	if strings.Contains(descLower, queryLower) {
		score += 50
	} else {
		queryWords := strings.Fields(queryLower)
		for _, word := range queryWords {
			if word != "" && strings.Contains(descLower, word) {
				score += 10
			}
			if word != "" && strings.Contains(argLower, word) {
				score += 5
			}
		}
	}

	return score
}
