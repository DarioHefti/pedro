package pageindex

import (
	"regexp"
	"strings"
)

// Chunk is a searchable piece of a page, labelled with its nearest heading.
type Chunk struct {
	Heading string
	Text    string
	Offset  int // byte offset in the Markdown where the chunk (or its heading) starts
}

const (
	targetChunkChars = 1200
	maxChunkChars    = 2000
)

// ChunkMarkdown splits Markdown into heading-labelled chunks of roughly
// targetChunkChars, never splitting inside a paragraph unless it is longer
// than maxChunkChars.
func ChunkMarkdown(md string) []Chunk {
	var chunks []Chunk
	heading, headingPos := "", -1
	var buf strings.Builder
	start := 0

	flush := func() {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text != "" {
			chunks = append(chunks, Chunk{Heading: heading, Text: text, Offset: start})
		}
	}

	pos := 0
	for _, raw := range strings.Split(md, "\n\n") {
		paraPos := pos
		pos += len(raw) + len("\n\n")
		para := strings.TrimSpace(raw)
		if para == "" {
			continue
		}
		if h, ok := headingText(para); ok {
			flush()
			heading, headingPos = h, paraPos+strings.Index(raw, para)
			continue
		}
		cursor := paraPos
		for _, piece := range splitLong(para, maxChunkChars) {
			piecePos := cursor
			if i := strings.Index(md[cursor:], piece); i >= 0 {
				piecePos = cursor + i
				cursor = piecePos + len(piece)
			}
			if buf.Len() > 0 && buf.Len()+len(piece) > targetChunkChars {
				flush()
			}
			if buf.Len() == 0 {
				start = piecePos
				if headingPos >= 0 { // the first chunk of a section starts at its heading
					start, headingPos = headingPos, -1
				}
			} else {
				buf.WriteString("\n\n")
			}
			buf.WriteString(piece)
		}
	}
	flush()
	return chunks
}

// headingText returns the text of a single-line Markdown ATX heading.
func headingText(para string) (string, bool) {
	if !strings.HasPrefix(para, "#") || strings.Contains(para, "\n") {
		return "", false
	}
	text := strings.TrimLeft(para, "#")
	text = markdownLink.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("**", "", "__", "", "*", "").Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	return text, text != ""
}

var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// splitLong cuts text into pieces of at most limit bytes, preferring
// sentence ends, then whitespace.
func splitLong(text string, limit int) []string {
	var pieces []string
	for len(text) > limit {
		cut := strings.LastIndex(text[:limit], ". ")
		if cut < limit/2 {
			cut = strings.LastIndexAny(text[:limit], " \n")
		}
		if cut <= 0 {
			cut = limit
			for cut > 0 && !isRuneStart(text[cut]) {
				cut--
			}
		} else {
			cut++
		}
		pieces = append(pieces, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	if text != "" {
		pieces = append(pieces, text)
	}
	return pieces
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
