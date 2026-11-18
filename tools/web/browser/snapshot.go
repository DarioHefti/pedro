package browser

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

// interactiveRoles get a ref so the agent can act on them.
var interactiveRoles = map[string]bool{
	"button": true, "link": true, "textbox": true, "searchbox": true, "combobox": true,
	"listbox": true, "option": true, "checkbox": true, "radio": true, "switch": true,
	"slider": true, "spinbutton": true, "menuitem": true, "menuitemcheckbox": true,
	"menuitemradio": true, "tab": true, "treeitem": true,
}

// inputRoles are shown even without an accessible name.
var inputRoles = map[string]bool{"textbox": true, "searchbox": true, "combobox": true}

// structuralRoles are shown (when named, or always for dialogs) to give the
// agent its bearings; their children are indented below them.
var structuralRoles = map[string]bool{
	"banner": true, "navigation": true, "main": true, "contentinfo": true, "complementary": true,
	"form": true, "search": true, "region": true, "dialog": true, "alertdialog": true,
	"alert": true, "status": true, "table": true,
}

// snapshot is a compact, text-only view of a page's accessibility tree.
type snapshot struct {
	text      string
	refs      map[string]proto.DOMBackendNodeID
	truncated bool
}

const (
	maxSnapshotRefs  = 400
	maxSnapshotChars = 16000
	maxNameChars     = 120
)

// buildSnapshot renders the accessibility tree as an indented list. Every
// interactive element gets a ref like e7 that the agent passes back.
func buildSnapshot(nodes []*proto.AccessibilityAXNode) snapshot {
	snap := snapshot{refs: make(map[string]proto.DOMBackendNodeID)}
	if len(nodes) == 0 {
		return snap
	}
	byID := make(map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, len(nodes))
	for _, n := range nodes {
		byID[n.NodeID] = n
	}

	var sb strings.Builder
	var walk func(n *proto.AccessibilityAXNode, depth int)
	walk = func(n *proto.AccessibilityAXNode, depth int) {
		if snap.truncated {
			return
		}
		childDepth := depth
		if !n.Ignored {
			if line, isRef := describe(n); line != "" {
				if isRef {
					if len(snap.refs) >= maxSnapshotRefs {
						snap.truncated = true
						return
					}
					ref := fmt.Sprintf("e%d", len(snap.refs)+1)
					snap.refs[ref] = n.BackendDOMNodeID
					line += " [ref=" + ref + "]"
				}
				if sb.Len()+len(line) > maxSnapshotChars {
					snap.truncated = true
					return
				}
				sb.WriteString(strings.Repeat("  ", depth))
				sb.WriteString("- ")
				sb.WriteString(line)
				sb.WriteByte('\n')
				childDepth = depth + 1
			}
		}
		for _, id := range n.ChildIDs {
			if c, ok := byID[id]; ok {
				walk(c, childDepth)
			}
		}
	}
	walk(nodes[0], 0)
	snap.text = sb.String()
	return snap
}

// describe returns the snapshot line for a node and whether it is actionable.
func describe(n *proto.AccessibilityAXNode) (string, bool) {
	role := axString(n.Role)
	name := truncateRunes(strings.Join(strings.Fields(axString(n.Name)), " "), maxNameChars)

	switch {
	case interactiveRoles[role] && n.BackendDOMNodeID != 0:
		if name == "" && !inputRoles[role] {
			return "", false
		}
		line := role
		if name != "" {
			line += fmt.Sprintf(" %q", name)
		}
		if v := truncateRunes(axString(n.Value), maxNameChars); v != "" && v != name {
			line += fmt.Sprintf(" value=%q", v)
		}
		return line + states(n), true

	case role == "heading" && name != "":
		level := ""
		for _, p := range n.Properties {
			if p.Name == proto.AccessibilityAXPropertyNameLevel && p.Value != nil {
				level = fmt.Sprintf(" [level=%d]", p.Value.Value.Int())
			}
		}
		return fmt.Sprintf("heading %q%s", name, level), false

	case structuralRoles[role]:
		if name == "" && role != "dialog" && role != "alertdialog" && role != "main" {
			return "", false
		}
		if name == "" {
			return role, false
		}
		return fmt.Sprintf("%s %q", role, name), false
	}
	return "", false
}

// states renders the boolean ARIA states the agent needs to reason about forms.
func states(n *proto.AccessibilityAXNode) string {
	var out []string
	for _, p := range n.Properties {
		if p.Value == nil {
			continue
		}
		v := p.Value.Value
		switch p.Name {
		case proto.AccessibilityAXPropertyNameChecked, proto.AccessibilityAXPropertyNameSelected,
			proto.AccessibilityAXPropertyNameDisabled, proto.AccessibilityAXPropertyNameRequired:
			if s := v.Str(); s == "true" || s == "mixed" || v.Bool() {
				out = append(out, string(p.Name))
			}
		case proto.AccessibilityAXPropertyNameExpanded:
			out = append(out, fmt.Sprintf("expanded=%v", v.Bool()))
		}
	}
	if len(out) == 0 {
		return ""
	}
	return " [" + strings.Join(out, ", ") + "]"
}

func axString(v *proto.AccessibilityAXValue) string {
	if v == nil || v.Value.Nil() {
		return ""
	}
	return strings.TrimSpace(v.Value.Str())
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
