package glterm

import (
	"image"
	"testing"
)

// TestFallbackDrawsWhatTheFontLacks: Go Mono has no half-width katakana, the
// glyphs of a Matrix rain, so the cell comes from the fallback font when it has
// them, or else from kana.gz, and is not drawn as a box.
func TestFallbackDrawsWhatTheFontLacks(t *testing.T) {
	if fallback() == nil {
		t.Skip("no fallback font on this machine")
	}
	g, err := NewGrid(16)
	if err != nil {
		t.Fatal(err)
	}
	f := g.faceFor("ｱ", 0)
	if fb := g.fallbackFace; fb != nil {
		if _, ok := fb.GlyphAdvance('ｱ'); ok && f == g.faces[0] {
			t.Fatal("ｱ was drawn with Go Mono though the fallback font has it")
		}
	}
	if g.faceFor("A", 0) != g.faces[0] {
		t.Error("A was not drawn with the terminal's own font")
	}
	m := g.glyph("ｱ", 0, 1)
	if m.Bounds() != image.Rect(0, 0, g.CellW, g.CellH) || !inked(m) {
		t.Error("ｱ left its cell empty")
	}
	if string(m.Pix) == string(g.glyph("一", 0, 1).Pix) {
		t.Error("ｱ was drawn as the box for a missing glyph")
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

// TestKanaWithoutAFallbackFont: with no fallback font to find, as on Android,
// half-width katakana are still drawn, from kana.gz, and not as Go Mono's box.
func TestKanaWithoutAFallbackFont(t *testing.T) {
	g, err := NewGrid(16)
	if err != nil {
		t.Fatal(err)
	}
	g.fallbackDone, g.fallbackFace = true, nil
	kana, box := g.glyph("ｱ", 0, 1), g.glyph("一", 0, 1)
	if !inked(kana) {
		t.Fatal("ｱ left its cell empty")
	}
	if string(kana.Pix) == string(box.Pix) {
		t.Fatal("ｱ was drawn as the box for a missing glyph")
	}
	if kanaBitmap('ﾝ') == nil || kanaBitmap('A') != nil {
		t.Error("kana.gz does not cover exactly U+FF61 to U+FF9F")
	}
}
