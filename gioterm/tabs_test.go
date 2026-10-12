//go:build cgo && unix

package gioterm

import (
	"image"
	"testing"

	"github.com/0magnet/progterm/glterm"
)

func named(titles ...string) *tabs {
	ts := &tabs{}
	for _, t := range titles {
		ts.add(&tab{title: t}, false)
	}
	return ts
}

func TestTabsAddRemove(t *testing.T) {
	ts := named("a", "b", "c", "d")
	if ts.cur != 0 {
		t.Fatalf("a tab added behind leaves the first current, got %d", ts.cur)
	}
	ts.add(&tab{title: "e"}, true)
	if ts.active().title != "e" {
		t.Fatalf("a tab added in front is current, got %q", ts.active().title)
	}
	ts.pick(2) // c
	ts.remove(0)
	if ts.active().title != "c" {
		t.Fatalf("removing a tab before the current one keeps it current, got %q", ts.active().title)
	}
	ts.remove(ts.cur) // c goes, d takes its place
	if ts.active().title != "d" {
		t.Fatalf("the tab after a removed current one takes its place, got %q", ts.active().title)
	}
	ts.pick(len(ts.list) - 1) // e, the last
	ts.remove(ts.cur)
	if ts.active().title != "d" {
		t.Fatalf("removing the last current tab leaves the one before, got %q", ts.active().title)
	}
	ts.remove(0)
	ts.remove(0)
	if ts.active() != nil {
		t.Fatal("no tabs left, but one is current")
	}
}

func TestTabsStepPick(t *testing.T) {
	ts := named("a", "b", "c")
	ts.step(-1)
	if ts.cur != 2 {
		t.Fatalf("step back from the first wraps to the last, got %d", ts.cur)
	}
	ts.step(1)
	if ts.cur != 0 {
		t.Fatalf("step on from the last wraps to the first, got %d", ts.cur)
	}
	ts.pick(7)
	if ts.cur != 0 {
		t.Fatal("picking a tab that does not exist changed the current one")
	}
}

func TestLayoutStrip(t *testing.T) {
	ts := named("sh", "skywire visor")
	s := layoutStrip(ts.labels(), 10, 1000, 20)
	if got := s.tabs[0]; got != image.Rect(0, 0, 60, 20) {
		t.Fatalf("first label %v", got)
	}
	if i, plus := s.hit(65); i != 1 || plus {
		t.Fatalf("x 65 hits %d %v, want tab 1", i, plus)
	}
	if _, plus := s.hit(s.plus.Min.X + 1); !plus {
		t.Fatal("the new tab button is not hit")
	}
	// Too many labels for the width share it, and the button stays inside.
	ts = named("one", "two", "three", "four", "five", "six")
	s = layoutStrip(ts.labels(), 10, 200, 20)
	if s.plus.Max.X > 200 {
		t.Fatalf("the new tab button is off the strip at %d", s.plus.Max.X)
	}
	if titleOf(glterm.Tab{}, "/usr/bin/zsh") != "zsh" || titleOf(glterm.Tab{Title: "visor"}, "x") != "visor" {
		t.Fatal("titles")
	}
}
