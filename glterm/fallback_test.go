package glterm

import (
	"image"
	"testing"
)

// TestFallbackDrawsWhatTheFontLacks: Go Mono has no half-width katakana, the
// glyphs of a Matrix rain, so the cell must come from the fallback font and
// not be drawn as a box.
func TestFallbackDrawsWhatTheFontLacks(t *testing.T) {
	if fallback() == nil {
		t.Skip("no fallback font on this machine")
	}
	g, err := NewGrid(16)
	if err != nil {
		t.Fatal(err)
	}
	if g.faceFor("ｱ", 0) == g.faces[0] {
		t.Fatal("ｱ was drawn with Go Mono, which has no glyph for it")
	}
	if g.faceFor("A", 0) != g.faces[0] {
		t.Error("A was not drawn with the terminal's own font")
	}
	if m := g.glyph("ｱ", 0, 1); m.Bounds() != image.Rect(0, 0, g.CellW, g.CellH) || !inked(m) {
		t.Error("ｱ left its cell empty")
	}
}

func inked(m *image.Alpha) bool {
	for _, a := range m.Pix {
		if a != 0 {
			return true
		}
	}
	return false
}
