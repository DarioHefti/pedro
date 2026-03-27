package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const toolDiscoveryName = "tool_discovery"

// ToolDiscoveryTool exposes a single, compact entrypoint to discover and execute
// other tools on demand.
type ToolDiscoveryTool struct {
	registry *Registry
}

func NewToolDiscoveryTool(registry *Registry) *ToolDiscoveryTool {
	return &ToolDiscoveryTool{registry: registry}
}

func (t ToolDiscoveryTool) Definition() Definition {
	return Definition{
		Name: toolDiscoveryName,
		Description: "Discover available tools and execute one by name. " +
			"Use action=list to see options, action=describe for one tool schema, and action=execute to run a tool.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "One of: list, describe, execute",
					"enum":        []string{"list", "describe", "execute"},
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Optional hint used by action=list to rank/filter tools by relevance",
				},
				"tool_name": map[string]any{
					"type":        "string",
					"description": "Tool to describe or execute",
				},
				"arguments": map[string]any{
					"type":        "object",
					"description": "JSON arguments to pass when action=execute",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t ToolDiscoveryTool) Execute(argsJSON string) (string, error) {
	if t.registry == nil {
		return "tool_discovery error: tool registry is not configured", nil
	}

	var args struct {
		Action    string          `json:"action"`
		Query     string          `json:"query"`
		ToolName  string          `json:"tool_name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	switch strings.ToLower(strings.TrimSpace(args.Action)) {
	case "list":
		return t.list(args.Query), nil
	case "describe":
		return t.describe(args.ToolName), nil
	case "execute":
		return t.execute(args.ToolName, args.Arguments), nil
	default:
		return "tool_discovery error: action must be one of list, describe, execute", nil
	}
}

func (t ToolDiscoveryTool) list(query string) string {
	defs := t.discoverableDefinitions()
	if len(defs) == 0 {
		return "No discoverable tools are registered."
	}

	type scoredDefinition struct {
		def   Definition
		score int
	}
	scored := make([]scoredDefinition, 0, len(defs))
	q := strings.ToLower(strings.TrimSpace(query))
	for _, def := range defs {
		score := 1
		if q != "" {
			name := strings.ToLower(def.Name)
			desc := strings.ToLower(def.Description)
			switch {
			case strings.Contains(name, q):
				score = 100
			case strings.Contains(desc, q):
				score = 50
			default:
				score = 0
				for _, token := range strings.Fields(q) {
					if token == "" {
						continue
					}
					if strings.Contains(name, token) {
						score += 8
					}
					if strings.Contains(desc, token) {
						score += 3
					}
				}
			}
		}
		if q == "" || score > 0 {
			scored = append(scored, scoredDefinition{def: def, score: score})
		}
	}

	if len(scored) == 0 {
		return fmt.Sprintf("No tools matched query %q. Use action=list without query to see everything.", query)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].def.Name < scored[j].def.Name
	})

	var sb strings.Builder
	sb.WriteString("Discovered tools:\n")
	for _, s := range scored {
		fmt.Fprintf(&sb, "- %s: %s\n", s.def.Name, s.def.Description)
	}
	sb.WriteString("\nNext steps:\n")
	sb.WriteString("- Use action=describe with tool_name for input schema.\n")
	sb.WriteString("- Use action=execute with tool_name + arguments to run a tool.\n")
	return sb.String()
}

func (t ToolDiscoveryTool) describe(toolName string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "tool_discovery error: tool_name is required for action=describe"
	}
	if name == toolDiscoveryName {
		return "tool_discovery cannot describe itself. Use action=list to view available tools."
	}
	tool, ok := t.registry.tools[name]
	if !ok {
		return fmt.Sprintf("Unknown tool: %s", name)
	}
	def := tool.Definition()
	params, err := json.MarshalIndent(def.Parameters, "", "  ")
	if err != nil {
		return fmt.Sprintf("Failed to encode schema for %s: %v", name, err)
	}
	return fmt.Sprintf("Tool: %s\nDescription: %s\nParameters schema:\n%s", def.Name, def.Description, string(params))
}

func (t ToolDiscoveryTool) execute(toolName string, args json.RawMessage) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "tool_discovery error: tool_name is required for action=execute"
	}
	if name == toolDiscoveryName {
		return "tool_discovery error: recursive execution is not allowed"
	}
	if _, ok := t.registry.tools[name]; !ok {
		return fmt.Sprintf("Unknown tool: %s", name)
	}

	if len(args) == 0 {
		args = []byte("{}")
	}
	return t.registry.Execute(name, string(args))
}

func (t ToolDiscoveryTool) discoverableDefinitions() []Definition {
	all := t.registry.AllDefinitions()
	out := make([]Definition, 0, len(all))
	for _, def := range all {
		if def.Name == toolDiscoveryName {
			continue
		}
		out = append(out, def)
	}
	return out
}
