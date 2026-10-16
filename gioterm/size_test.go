//go:build cgo && unix

package gioterm

import (
	"image"
	"testing"

	"github.com/0magnet/progterm/glterm"
)

func TestWindowSize(t *testing.T) {
	g := &glterm.Grid{CellW: 8, CellH: 16}
	strip := 16 + 16/3
	if got, want := windowSize(g, 100, 30, image.Point{}), image.Pt(800, 30*16+strip); got != want {
		t.Errorf("unknown screen: %v, want %v", got, want)
	}
	if got := windowSize(g, 100, 30, image.Pt(640, 480)); got.X > 576 || got.Y > 408 || got.Y >= got.X {
		t.Errorf("small screen: %v", got)
	}
	// A tall font on a tall screen still opens wider than tall.
	tall := &glterm.Grid{CellW: 8, CellH: 40}
	if got := windowSize(tall, 30, 30, image.Pt(1080, 1920)); got.Y >= got.X {
		t.Errorf("portrait: %v", got)
	}
}
