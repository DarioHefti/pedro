package tools

import (
	"fmt"
	"net/http"
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
	tools map[string]Tool
	order []string // preserves registration order for tool definitions
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	name := t.Definition().Name
	r.tools[name] = t
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
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// New builds and returns a Registry pre-loaded with all available tools.
// Add or remove Register calls here to enable/disable tools at runtime.
func New() *Registry {
	r := NewRegistry()
	r.Register(NewSearchTool())
	r.Register(NewFetchURLTool())
	r.Register(NewReadFileTool())
	return r
}
