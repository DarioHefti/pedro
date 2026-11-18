package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pedro/tools/web/browser"
	"pedro/tools/web/extract"
	"pedro/tools/web/pageindex"
)

// BrowserTool lets the model drive a real (headless, local) Chrome tab:
// open pages, click links and buttons, fill in forms and read the result.
type BrowserTool struct {
	session   browserSession
	extractor extract.Extractor
	index     pageindex.Index
}

func NewBrowserTool(session browserSession, extractor extract.Extractor, index pageindex.Index) *BrowserTool {
	return &BrowserTool{session: session, extractor: extractor, index: index}
}

var browserActions = []string{"navigate", "snapshot", "click", "type", "select", "press", "scroll", "back", "read", "close"}

func (BrowserTool) Definition() Definition {
	return Definition{
		Name: "browser",
		Description: "Control a web browser to interact with websites: navigate, click links and buttons, fill in and submit forms, use a site's own search, open menus or accept cookie banners. " +
			"Every action returns a snapshot listing the page's headings and interactive elements, each with a ref (e.g. e12). Use those refs with click, type and select; refs change after every action. " +
			"Use read to get the page content as Markdown. For simply reading a known URL prefer webclaw.",
		DeferLoading: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": browserActions,
					"description": "navigate (url), snapshot, click (ref), type (ref, text, optional submit), select (ref, text = option label), " +
						"press (key, e.g. Enter, Escape, ArrowDown), scroll (direction up/down), back, read (page content as Markdown), close",
				},
				"url":       map[string]any{"type": "string", "description": "URL for navigate"},
				"ref":       map[string]any{"type": "string", "description": "Element ref from the latest snapshot, e.g. e12"},
				"text":      map[string]any{"type": "string", "description": "Text to type, or the option to select"},
				"submit":    map[string]any{"type": "boolean", "description": "Press Enter after typing"},
				"key":       map[string]any{"type": "string", "description": "Key for press"},
				"direction": map[string]any{"type": "string", "enum": []string{"down", "up"}, "description": "Scroll direction (default down)"},
			},
			"required": []string{"action"},
		},
	}
}

func (t BrowserTool) Execute(argsJSON string) (string, error) {
	var args struct {
		Action    string `json:"action"`
		URL       string `json:"url"`
		Ref       string `json:"ref"`
		Text      string `json:"text"`
		Submit    bool   `json:"submit"`
		Key       string `json:"key"`
		Direction string `json:"direction"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var state *browser.State
	var err error
	switch strings.ToLower(strings.TrimSpace(args.Action)) {
	case "navigate":
		if strings.TrimSpace(args.URL) == "" {
			return "Error: navigate needs a url", nil
		}
		state, err = t.session.Navigate(ctx, args.URL)
	case "snapshot":
		state, err = t.session.Snapshot(ctx)
	case "click":
		state, err = t.session.Click(ctx, args.Ref)
	case "type":
		state, err = t.session.Type(ctx, args.Ref, args.Text, args.Submit)
	case "select":
		state, err = t.session.Select(ctx, args.Ref, args.Text)
	case "press":
		state, err = t.session.Press(ctx, args.Key)
	case "scroll":
		state, err = t.session.Scroll(ctx, args.Direction)
	case "back":
		state, err = t.session.Back(ctx)
	case "read":
		return t.read(ctx), nil
	case "close":
		_ = t.session.Close()
		return "Browser tab closed.", nil
	default:
		return fmt.Sprintf("Error: unknown action %q; use one of %s", args.Action, strings.Join(browserActions, ", ")), nil
	}
	if err != nil {
		return fmt.Sprintf("Browser error: %v", err), nil
	}
	return formatBrowserState(state), nil
}

func (t BrowserTool) read(ctx context.Context) string {
	pageURL, html, err := t.session.HTML(ctx)
	if err != nil {
		return fmt.Sprintf("Browser error: %v", err)
	}
	article, err := t.extractor.Extract(html, pageURL)
	if err != nil {
		return fmt.Sprintf("Could not extract content: %v", err)
	}
	indexArticle(t.index, pageURL, article)
	return formatPagePart(pageURL, article, 0)
}

func formatBrowserState(s *browser.State) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "URL: %s\nTitle: %s\n\nPage snapshot:\n", s.URL, s.Title)
	if strings.TrimSpace(s.Snapshot) == "" {
		sb.WriteString("(no headings or interactive elements found; use read to get the page text)\n")
	} else {
		sb.WriteString(s.Snapshot)
	}
	if s.Truncated {
		sb.WriteString("\n[Snapshot truncated. Scroll, or use read for the page text.]")
	}
	if browser.IsChallengeTitle(s.Title) {
		sb.WriteString("\n\n[This is a bot check that asks for human verification. It cannot be passed automatically; use another source.]")
	}
	return strings.TrimSpace(sb.String())
}
