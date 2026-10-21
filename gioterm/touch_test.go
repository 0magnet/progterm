package gioterm

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/0magnet/gio/f32"
	"github.com/0magnet/gio/io/pointer"

	"github.com/0magnet/progterm/glterm"
)

func describe(es []glterm.MouseEvent) string {
	var s []string
	for _, e := range es {
		s = append(s, fmt.Sprintf("%v/%v@%v,%v", e.Button, e.Action, e.X, e.Y))
	}
	return strings.Join(s, " ")
}

func TestTouch(t *testing.T) {
	t0 := time.Unix(0, 0)
	ev := func(k pointer.Kind, x float32) pointer.Event {
		return pointer.Event{Kind: k, Source: pointer.Touch, Position: f32.Pt(x, 5)}
	}
	left, right := glterm.ButtonLeft, glterm.ButtonRight
	tests := []struct {
		name string
		run  func(*touch) []glterm.MouseEvent
		want []glterm.MouseEvent
	}{
		{"tap", func(tc *touch) []glterm.MouseEvent {
			tc.event(ev(pointer.Press, 10), t0, 8)
			return tc.event(ev(pointer.Release, 11), t0.Add(100*time.Millisecond), 8)
		}, []glterm.MouseEvent{{X: 10, Y: 5, Button: left, Action: glterm.Press}, {X: 11, Y: 5, Button: left, Action: glterm.Release}}},

		{"drag selects", func(tc *touch) []glterm.MouseEvent {
			tc.event(ev(pointer.Press, 10), t0, 8)
			out := tc.event(ev(pointer.Drag, 13), t0, 8) // within the slop
			out = append(out, tc.event(ev(pointer.Drag, 40), t0, 8)...)
			return append(out, tc.event(ev(pointer.Release, 40), t0, 8)...)
		}, []glterm.MouseEvent{{X: 10, Y: 5, Button: left, Action: glterm.Press}, {X: 40, Y: 5, Action: glterm.Move}, {X: 40, Y: 5, Action: glterm.Move}, {X: 40, Y: 5, Button: left, Action: glterm.Release}}},

		{"long press opens the menu and keeps the selection", func(tc *touch) []glterm.MouseEvent {
			tc.event(ev(pointer.Press, 10), t0, 8)
			if out := tc.expire(t0.Add(longPress - time.Millisecond)); out != nil {
				return out
			}
			out := tc.expire(t0.Add(longPress))
			out = append(out, tc.event(ev(pointer.Drag, 50), t0, 8)...)
			return append(out, tc.event(ev(pointer.Release, 50), t0, 8)...)
		}, []glterm.MouseEvent{{X: 10, Y: 5, Button: right, Action: glterm.Press}}},
	}
	for _, tt := range tests {
		var tc touch
		if got := tt.run(&tc); describe(got) != describe(tt.want) {
			t.Errorf("%s: sent %s, want %s", tt.name, describe(got), describe(tt.want))
		}
	}
}

func TestPinch(t *testing.T) {
	ev := func(k pointer.Kind, id pointer.ID, x float32) pointer.Event {
		return pointer.Event{Kind: k, Source: pointer.Touch, PointerID: id, Position: f32.Pt(x, 50)}
	}
	var p pinch
	var tc touch
	zoom := 1.0
	t0 := time.Unix(0, 0)

	// One finger starts a selection; the pinch leaves it alone.
	tc.event(ev(pointer.Press, 1, 100), t0, 8)
	if p.event(ev(pointer.Press, 1, 100), &zoom) {
		t.Fatal("one finger was taken for a pinch")
	}
	tc.event(ev(pointer.Drag, 1, 140), t0, 8)

	// A second finger makes it a pinch, and the selection is let go.
	if !p.event(ev(pointer.Press, 2, 200), &zoom) {
		t.Fatal("two fingers were not a pinch")
	}
	if got := describe(tc.abort()); got != describe([]glterm.MouseEvent{{X: 100, Y: 50, Button: glterm.ButtonLeft, Action: glterm.Release}}) {
		t.Errorf("the selection was not let go: %s", got)
	}

	// Fingers twice as far apart, 100 to 200 becoming 100 to 300.
	p.event(ev(pointer.Drag, 2, 300), &zoom)
	if zoom < 1.9 || zoom > 2.1 {
		t.Errorf("fingers twice as far apart gave zoom %v", zoom)
	}

	// The pinch keeps the touch until the last finger is up.
	if !p.event(ev(pointer.Release, 2, 300), &zoom) || !p.event(ev(pointer.Drag, 1, 120), &zoom) {
		t.Error("the finger left after a pinch was let through")
	}
	if !p.event(ev(pointer.Release, 1, 120), &zoom) || p.on {
		t.Error("the pinch did not end with the last finger")
	}
	if p.event(ev(pointer.Press, 3, 10), &zoom) {
		t.Error("a new touch after the pinch was taken by it")
	}
}
