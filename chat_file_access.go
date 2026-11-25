package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pedro/tools"
)

// fileAccessFor is what the file tools may read during a reply in this chat: the files and
// folders (with everything below them) the person attached or wrote the path of in their own
// messages, plus Pedro's own image and transcript folders. Tool results never add to it, so a
// web page cannot talk the model into reading anything else on the computer.
func (a *App) fileAccessFor(messages []Message) tools.FileAccess {
	files, dirs := userGivenPaths(messages)
	return tools.NewAllowList(files, append(dirs, a.appFileDirs...))
}

// userGivenPaths collects the files and folders the person's messages point to.
func userGivenPaths(messages []Message) (files, dirs []string) {
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		var atts []struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if m.Attachments != "" && json.Unmarshal([]byte(m.Attachments), &atts) == nil {
			for _, att := range atts {
				switch att.Type {
				case "file-ref":
					files = append(files, att.Content)
				case "folder-ref":
					dirs = append(dirs, att.Content)
				}
			}
		}
		for _, p := range writtenPaths(m.Content) {
			info, err := os.Stat(p)
			switch {
			case err != nil:
			case !info.IsDir():
				files = append(files, p)
			// A path mentioned in passing ("my home is /Users/me") must not open up the whole
			// disk; a broad folder has to be attached on purpose.
			case !tools.IsBroadFolder(p):
				dirs = append(dirs, p)
			}
		}
	}
	return files, dirs
}

var (
	// A path in quotes or backticks, which may contain spaces.
	quotedPathRe = regexp.MustCompile("[\"'`]((?:~[/\\\\]|/|[A-Za-z]:[/\\\\])[^\"'`\\n]+)[\"'`]")
	// A "[Path: …]" note line, as the frontend adds for attachments.
	bracketPathLineRe = regexp.MustCompile(`(?mi)^[ \t]*\[path:.*\][ \t]*$`)
	// A path standing on its own: at the start of a line or after a space or bracket.
	barePathRe = regexp.MustCompile("(?m)(?:^|[\\s(\\[<])((?:~[/\\\\]|/|[A-Za-z]:[/\\\\])[^\\s\"'`<>()\\[\\]]+)")
)

// writtenPaths finds absolute paths written in a message, e.g. "summarize ~/notes/todo.md"
// or the "[Path: …]" notes of attachments. "~" stands for the home folder.
func writtenPaths(content string) []string {
	if content == "" {
		return nil
	}
	candidates := extractBracketPathMarkers(content)
	// The notes are read whole above; matched again below, a path with spaces would come out
	// cut short at the first space, naming a broader folder than the one attached.
	rest := bracketPathLineRe.ReplaceAllString(content, "")
	for _, re := range []*regexp.Regexp{quotedPathRe, barePathRe} {
		for _, m := range re.FindAllStringSubmatch(rest, -1) {
			candidates = append(candidates, m[1])
		}
	}

	home, _ := os.UserHomeDir()
	var out []string
	seen := map[string]bool{}
	for _, p := range candidates {
		p = strings.TrimRight(strings.TrimSpace(p), ".,;:!?")
		if rest, ok := strings.CutPrefix(p, "~"); ok {
			if home == "" {
				continue
			}
			p = filepath.Join(home, rest)
		}
		if !filepath.IsAbs(p) {
			continue
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
