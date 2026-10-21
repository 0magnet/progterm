//go:build ignore

// Command kanagen writes glterm's kana.gz: the half-width katakana forms,
// U+FF61 to U+FF9F, drawn from a font that has them, for a terminal whose own
// fonts do not. Go Mono has none, and Android's CJK font is CID-keyed CFF,
// which x/image cannot read.
//
//	go run ./internal/kanagen/main.go /usr/share/fonts/adobe-source-han-sans/SourceHanSansJP-Regular.otf
//
// The output is the glyph width and height, a byte each, then each glyph's
// alpha in rows, gzipped. A glyph is its em box, the baseline at 0.88. Drawn from Source Han Sans, it is under the SIL Open
// Font License 1.1, as kana.LICENSE says.
package main

import (
	"compress/gzip"
	"fmt"
	"image"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func main() {
	if err := run(os.Args[1], "kana.gz"); err != nil {
		fmt.Fprintln(os.Stderr, "kanagen:", err)
		os.Exit(1)
	}
}

func run(fontFile, out string) error {
	data, err := os.ReadFile(fontFile) //nolint:gosec // the font named
	if err != nil {
		return err
	}
	c, err := opentype.ParseCollection(data)
	if err != nil {
		return err
	}
	f, err := c.Font(0)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 56, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return err
	}
	adv, ok := face.GlyphAdvance('ｱ')
	if !ok {
		return fmt.Errorf("%s has no half-width katakana", fontFile)
	}
	// The em box, its baseline 0.88 down as in CJK fonts, rather than the
	// font's line, which leaves room far above and below.
	w, h := adv.Ceil(), 56
	baseline := 49
	o, err := os.Create(out)
	if err != nil {
		return err
	}
	z, _ := gzip.NewWriterLevel(o, gzip.BestCompression) //nolint:errcheck // a valid level
	z.Write([]byte{byte(w), byte(h)})                    //nolint:errcheck,gosec // checked at Close
	for r := rune(0xff61); r <= 0xff9f; r++ {
		img := image.NewAlpha(image.Rect(0, 0, w, h))
		d := font.Drawer{Dst: img, Src: image.Opaque, Face: face, Dot: fixed.P(0, baseline)}
		d.DrawString(string(r))
		z.Write(img.Pix) //nolint:errcheck,gosec // as above
	}
	if err := z.Close(); err != nil {
		return err
	}
	return o.Close()
}
