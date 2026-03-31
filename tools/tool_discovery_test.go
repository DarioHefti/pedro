package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRegistryExposesToolSearch(t *testing.T) {
	r := New()
	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected exactly 1 exposed tool, got %d", len(defs))
	}
	if defs[0].Name != ToolSearchToolName {
		t.Fatalf("expected exposed tool %q, got %q", ToolSearchToolName, defs[0].Name)
	}

	all := r.AllDefinitions()
	if len(all) < 2 {
		t.Fatalf("expected hidden tools to be registered too, got %d total", len(all))
	}
}

func TestToolDiscoveryListDescribeAndExecute(t *testing.T) {
	r := New()
	d := NewToolDiscoveryTool(r)

	listPayload, _ := json.Marshal(map[string]any{
		"action": "list",
	})
	listOut, err := d.Execute(string(listPayload))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, "read_file") {
		t.Fatalf("list output should include discoverable tools: %q", listOut)
	}

	describePayload, _ := json.Marshal(map[string]any{
		"action":    "describe",
		"tool_name": "read_file",
	})
	describeOut, err := d.Execute(string(describePayload))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(describeOut, "Tool: read_file") {
		t.Fatalf("unexpected describe output: %q", describeOut)
	}

	execPayload, _ := json.Marshal(map[string]any{
		"action":    "execute",
		"tool_name": "read_file",
		"arguments": map[string]any{},
	})
	execOut, err := d.Execute(string(execPayload))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(execOut, "File read error") {
		t.Fatalf("expected read_file argument error, got: %q", execOut)
	}
}
