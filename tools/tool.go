package tools

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

// UserAgent is the browser UA sent with every outbound HTTP request.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"

// HTTPClient is shared by all tools.
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// Definition describes a tool's JSON schema in a provider-agnostic way.
type Definition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Tool is the interface every tool must implement.
type Tool interface {
	Definition() Definition
	// Execute receives the raw JSON arguments string and returns the result.
	Execute(argsJSON string) (string, error)
}

// Registry holds all registered tools and dispatches calls to them.
type Registry struct {
	tools   map[string]Tool
	visible map[string]bool
	order   []string // preserves registration order for tool definitions
}

func NewRegistry() *Registry {
	return &Registry{
		tools:   make(map[string]Tool),
		visible: make(map[string]bool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.registerWithVisibility(t, true)
}

// RegisterHidden adds a tool that can be executed but is not exposed directly
// to the model tool list.
func (r *Registry) RegisterHidden(t Tool) {
	r.registerWithVisibility(t, false)
}

func (r *Registry) registerWithVisibility(t Tool, visible bool) {
	name := t.Definition().Name
	r.tools[name] = t
	r.visible[name] = visible
	for _, existing := range r.order {
		if existing == name {
			return
		}
	}
	r.order = append(r.order, name)
}

// Execute dispatches a tool call by name with the given JSON arguments.
// Returns a user-visible error string (never a Go error) so the LLM always
// gets a response even on failure.
func (r *Registry) Execute(name, argsJSON string) string {
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("Unknown tool: %s", name)
	}
	result, err := t.Execute(argsJSON)
	if err != nil {
		return fmt.Sprintf("Tool %s error: %v", name, err)
	}
	return result
}

// Definitions returns all tool definitions in registration order.
func (r *Registry) Definitions() []Definition {
	defs := make([]Definition, 0, len(r.order))
	for _, name := range r.order {
		if !r.visible[name] {
			continue
		}
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// AllDefinitions returns all tool definitions (including hidden tools) in
// registration order.
func (r *Registry) AllDefinitions() []Definition {
	defs := make([]Definition, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// DefinitionByName returns a single tool definition by tool name.
func (r *Registry) DefinitionByName(name string) (Definition, bool) {
	t, ok := r.tools[name]
	if !ok {
		return Definition{}, false
	}
	return t.Definition(), true
}

// DefinitionsByName returns definitions for known names in deterministic order.
func (r *Registry) DefinitionsByName(names []string) []Definition {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	out := make([]Definition, 0, len(set))
	for _, n := range r.order {
		if _, ok := set[n]; !ok {
			continue
		}
		if def, exists := r.DefinitionByName(n); exists {
			out = append(out, def)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// New builds and returns a Registry pre-loaded with all available tools.
// Add or remove Register calls here to enable/disable tools at runtime.
func New() *Registry {
	r := NewRegistry()
	r.RegisterHidden(NewSearchTool())
	r.RegisterHidden(NewFetchURLTool())
	r.RegisterHidden(NewReadFileTool())
	r.RegisterHidden(NewParseDocumentTool())
	r.RegisterHidden(NewShowFileTreeTool())
	r.Register(NewToolDiscoveryTool(r))
	return r
}
