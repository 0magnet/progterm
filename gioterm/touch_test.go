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
