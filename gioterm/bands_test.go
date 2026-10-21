package gioterm

import (
	"image"
	"image/color"
	"testing"

	"github.com/0magnet/gio/op"
)

// TestBandsKeepUnchanged checks that only a changed band gets a new ImageOp,
// which is what makes Gio upload a texture.
func TestBandsKeepUnchanged(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 10)) // bands of 4, 4 and 2 rows
	var b bands
	var ops op.Ops
	b.paint(&ops, img, image.Point{}, 4)
	if len(b.ops) != 3 {
		t.Fatalf("%d bands, want 3", len(b.ops))
	}
	first := append([]*byte(nil), &b.pix[0][0], &b.pix[1][0], &b.pix[2][0])

	img.Set(3, 5, color.RGBA{R: 0xff, A: 0xff}) // in the middle band
	ops.Reset()
	b.paint(&ops, img, image.Point{}, 4)
	for i, want := range []bool{false, true, false} {
		if changed := &b.pix[i][0] != first[i]; changed != want {
			t.Errorf("band %d replaced = %v, want %v", i, changed, want)
		}
	}

	ops.Reset()
	b.paint(&ops, image.NewRGBA(image.Rect(0, 0, 8, 12)), image.Point{}, 4)
	if len(b.ops) != 3 || b.size != image.Pt(8, 12) {
		t.Errorf("after a resize: %d bands at %v", len(b.ops), b.size)
	}
}
