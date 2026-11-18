package pageindex

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func newIndex(t *testing.T, maxPages int) *SQLiteIndex {
	t.Helper()
	x, err := NewSQLiteIndex(maxPages)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	return x
}

const law = `# Obligationenrecht

## Art. 335c

1 Das Arbeitsverhältnis kann im ersten Dienstjahr mit einer Kündigungsfrist von einem Monat gekündigt werden.

## Art. 336

1 Die Kündigung eines Arbeitsverhältnisses ist missbräuchlich, wenn eine Partei sie ausspricht.`

func TestSearchFindsPassageWithHeading(t *testing.T) {
	x := newIndex(t, 10)
	if err := x.Add(Page{URL: "https://www.fedlex.admin.ch/eli/cc/27/317_321_377/de", Title: "OR", Markdown: law}); err != nil {
		t.Fatal(err)
	}
	if err := x.Add(Page{URL: "https://example.com/other", Title: "Other", Markdown: "Nothing about employment here."}); err != nil {
		t.Fatal(err)
	}

	// Prefix + diacritics: "kundigungsfrist" matches "Kündigungsfrist".
	hits, err := x.Search(Query{Text: "kundigungsfrist erstes Dienstjahr"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Heading != "Art. 335c" || !strings.Contains(hits[0].Text, "einem Monat") {
		t.Fatalf("unexpected hits %+v", hits)
	}

	hits, _ = x.Search(Query{Text: "Kündigung", Host: "example.com"})
	if len(hits) != 0 {
		t.Errorf("host filter should exclude fedlex, got %+v", hits)
	}
	hits, _ = x.Search(Query{Text: "Kündigung", URL: "http://fedlex.admin.ch/eli/cc/27/317_321_377/de/"})
	if len(hits) != 2 {
		t.Errorf("URL filter should match the normalized URL, got %d hits", len(hits))
	}
}

func TestSearchMatchesSplitArticleNumbers(t *testing.T) {
	x := newIndex(t, 10)
	md := "###### [**Art. 335** *b*](https://x.ch/#b)\n\nProbezeit von sieben Tagen.\n\n" +
		"###### [**Art. 335** *c*](https://x.ch/#c)\n\nIm ersten Dienstjahr ein Monat.\n\n" +
		"###### Art. 1098\n\nVerweis auf Art. 335 und andere Artikel."
	if err := x.Add(Page{URL: "https://x.ch/or", Markdown: md}); err != nil {
		t.Fatal(err)
	}
	hits, err := x.Search(Query{Text: "Art. 335c"})
	if err != nil || len(hits) == 0 || hits[0].Heading != "Art. 335 c" {
		t.Fatalf("expected Art. 335 c first, got %+v err=%v", hits, err)
	}
	if !strings.HasPrefix(md[hits[0].Offset:], "###### [**Art. 335** *c*]") {
		t.Errorf("offset should point at the section heading, got %q", md[hits[0].Offset:hits[0].Offset+20])
	}
	if got := splitLettersDigits("art335c"); got != "art 335 c" {
		t.Errorf("splitLettersDigits = %q", got)
	}
}

func TestAddReplacesAndEvicts(t *testing.T) {
	x := newIndex(t, 2)
	for i := 0; i < 3; i++ {
		if err := x.Add(Page{URL: fmt.Sprintf("https://a.com/%d", i), Markdown: fmt.Sprintf("page number %d unique%d", i, i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := x.Get("https://a.com/0", time.Hour); ok {
		t.Error("oldest page should have been evicted")
	}
	if hits, _ := x.Search(Query{Text: "unique0"}); len(hits) != 0 {
		t.Error("chunks of evicted pages must be removed")
	}

	if err := x.Add(Page{URL: "https://a.com/2", Markdown: "replaced content"}); err != nil {
		t.Fatal(err)
	}
	p, ok := x.Get("https://a.com/2", time.Hour)
	if !ok || p.Markdown != "replaced content" {
		t.Fatalf("expected replaced page, got %+v", p)
	}
	if hits, _ := x.Search(Query{Text: "unique2"}); len(hits) != 0 {
		t.Error("old chunks of a replaced page must be removed")
	}
	if n := x.CountHost("www.a.com"); n != 2 {
		t.Errorf("CountHost = %d, want 2", n)
	}
}

func TestGetRespectsMaxAge(t *testing.T) {
	x := newIndex(t, 5)
	_ = x.Add(Page{URL: "https://a.com/", Markdown: "x"})
	if _, ok := x.Get("https://a.com/", -time.Second); ok {
		t.Error("expired page should not be returned")
	}
}

func TestSearchRejectsQueriesWithoutWords(t *testing.T) {
	x := newIndex(t, 5)
	if _, err := x.Search(Query{Text: `"*) - ( !`}); err == nil {
		t.Error("expected an error for a query without words")
	}
}

func TestHeadingText(t *testing.T) {
	got, ok := headingText("###### [**Art. 335** *b*](https://x.ch/#art_335_b) [186](https://x.ch/#fn)")
	if !ok || got != "Art. 335 b 186" {
		t.Errorf("headingText = %q", got)
	}
	if _, ok := headingText("#hashtag\nsecond line"); ok {
		t.Error("multi-line paragraphs are not headings")
	}
}

func TestChunkMarkdown(t *testing.T) {
	long := strings.Repeat("Ein Satz über das Mietrecht. ", 200) // ~5800 bytes, one paragraph
	md := "intro\n\n## **Heading**\n\n" + long
	chunks := ChunkMarkdown(md)
	for i, c := range chunks[1:] {
		if i == 0 && !strings.HasPrefix(md[c.Offset:], "## **Heading**") {
			t.Errorf("first section chunk should start at its heading")
		}
		if i > 0 && !strings.HasPrefix(md[c.Offset:], c.Text[:30]) {
			t.Errorf("chunk %d offset %d does not point at its text", i+1, c.Offset)
		}
	}
	if chunks[0].Heading != "" || chunks[0].Text != "intro" {
		t.Errorf("first chunk = %+v", chunks[0])
	}
	for _, c := range chunks[1:] {
		if c.Heading != "Heading" {
			t.Errorf("chunk heading = %q", c.Heading)
		}
		if len(c.Text) > maxChunkChars {
			t.Errorf("chunk of %d bytes exceeds limit", len(c.Text))
		}
		if !strings.HasSuffix(c.Text, ".") {
			t.Errorf("chunk should end at a sentence boundary: %q", c.Text[len(c.Text)-20:])
		}
	}
}
