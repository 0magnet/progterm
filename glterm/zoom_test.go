package glterm

import (
	"math"
	"testing"
)

func TestZoomKey(t *testing.T) {
	for _, c := range []struct {
		e    KeyEvent
		want float64
		ok   bool
	}{
		{KeyEvent{Rune: '=', Mods: Ctrl}, 1.1, true},
		{KeyEvent{Rune: '+', Mods: Ctrl | Shift}, 1.1, true},
		{KeyEvent{Rune: '-', Mods: Ctrl}, 1 / 1.1, true},
		{KeyEvent{Rune: '0', Mods: Ctrl}, 1, true},
		{KeyEvent{Rune: '='}, 1, false},                   // typed, not a zoom
		{KeyEvent{Rune: '=', Mods: Ctrl | Alt}, 1, false}, // left to the program
		{KeyEvent{Rune: 'c', Mods: Ctrl}, 1, false},
	} {
		got, ok := ZoomKey(c.e, 1)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%+v: zoom %v %v, want %v %v", c.e, got, ok, c.want, c.ok)
		}
	}
	if got, _ := ZoomKey(KeyEvent{Rune: '=', Mods: Ctrl}, MaxZoom); got != MaxZoom {
		t.Errorf("zoom past MaxZoom: %v", got)
	}
}

func TestPinchZoom(t *testing.T) {
	if got := PinchZoom(1, 100, 200); math.Abs(got-math.Pow(pinchStep, 14)) > 1e-9 {
		t.Errorf("fingers twice as far apart: zoom %v, want the step nearest 2", got)
	}
	if got := PinchZoom(1, 100, 102); got != 1 {
		t.Errorf("a 2%% pinch moved the zoom to %v, under half a step", got)
	}
	if got := PinchZoom(1, 100, 1); got != MinZoom {
		t.Errorf("a pinch to nothing: zoom %v, want %v", got, MinZoom)
	}
}
