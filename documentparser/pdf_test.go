package documentparser

import (
	"testing"

	"github.com/ledongthuc/pdf"
)

func TestJoinGlyphsRebuildsSpacesAndLines(t *testing.T) {
	glyph := func(s string, x, y, w float64) pdf.Text {
		return pdf.Text{S: s, X: x, Y: y, W: w, FontSize: 12}
	}
	glyphs := []pdf.Text{
		glyph("A", 100, 700, 6), glyph("V", 105.8, 700, 6), // kerned pair, no space
		glyph("i", 115, 700, 3), glyph("s", 118, 700, 4), // 3pt gap: a word space
		glyph("x", 100, 686, 6),                          // next line
		glyph("2", 106, 690, 4),                          // superscript stays on the line
		glyph(" ", 110, 686, 3), glyph("y", 113, 686, 6), // explicit space
	}
	if got, want := joinGlyphs(glyphs), "AV is\nx2 y"; got != want {
		t.Errorf("joinGlyphs = %q, want %q", got, want)
	}
}
