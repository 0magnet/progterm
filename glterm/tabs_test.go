//go:build unix

package glterm

import (
	"image"
	"testing"
)

func named(titles ...string) *Tabs {
	ts := &Tabs{}
	for _, t := range titles {
		ts.Add(&Shown{Title: t}, false)
	}
	return ts
}

func TestTabsAddRemove(t *testing.T) {
	ts := named("a", "b", "c", "d")
	if ts.Cur != 0 {
		t.Fatalf("a tab added behind leaves the first current, got %d", ts.Cur)
	}
	ts.Add(&Shown{Title: "e"}, true)
	if ts.Active().Title != "e" {
		t.Fatalf("a tab added in front is current, got %q", ts.Active().Title)
	}
	ts.Pick(2) // c
	ts.Remove(0)
	if ts.Active().Title != "c" {
		t.Fatalf("removing a tab before the current one keeps it current, got %q", ts.Active().Title)
	}
	ts.Remove(ts.Cur) // c goes, d takes its place
	if ts.Active().Title != "d" {
		t.Fatalf("the tab after a removed current one takes its place, got %q", ts.Active().Title)
	}
	ts.Pick(len(ts.List) - 1) // e, the last
	ts.Remove(ts.Cur)
	if ts.Active().Title != "d" {
		t.Fatalf("removing the last current tab leaves the one before, got %q", ts.Active().Title)
	}
	ts.Remove(0)
	ts.Remove(0)
	if ts.Active() != nil {
		t.Fatal("no tabs left, but one is current")
	}
}

func TestTabsStepPick(t *testing.T) {
	ts := named("a", "b", "c")
	ts.Step(-1)
	if ts.Cur != 2 {
		t.Fatalf("step back from the first wraps to the last, got %d", ts.Cur)
	}
	ts.Step(1)
	if ts.Cur != 0 {
		t.Fatalf("step on from the last wraps to the first, got %d", ts.Cur)
	}
	ts.Pick(7)
	if ts.Cur != 0 {
		t.Fatal("picking a tab that does not exist changed the current one")
	}
}

func TestTabsShortcut(t *testing.T) {
	ts := named("a", "b", "c")
	opened := 0
	newTab := func() { opened++ }
	for _, c := range []struct {
		e    KeyEvent
		want int
	}{
		{KeyEvent{Key: KeyTab, Mods: Ctrl}, 1},
		{KeyEvent{Key: KeyTab, Mods: Ctrl | Shift}, 0},
		{KeyEvent{Key: KeyPageUp, Mods: Ctrl}, 2},
		{KeyEvent{Rune: '2', Mods: Alt}, 1},
	} {
		if !ts.Shortcut(c.e, newTab) || ts.Cur != c.want {
			t.Fatalf("%+v: current %d, want %d", c.e, ts.Cur, c.want)
		}
	}
	if !ts.Shortcut(KeyEvent{Rune: 'T', Mods: Ctrl | Shift}, newTab) || opened != 1 {
		t.Fatal("Ctrl+Shift+T opened no tab")
	}
	if ts.Shortcut(KeyEvent{Rune: 't', Mods: Ctrl}, newTab) || ts.Shortcut(KeyEvent{Rune: 'x'}, newTab) {
		t.Fatal("a key that is no shortcut was taken")
	}
}

func TestLayoutStrip(t *testing.T) {
	ts := named("sh", "skywire visor")
	s := LayoutStrip(ts.Labels(), 10, 1000, 20)
	if got := s.Tabs[0]; got != image.Rect(0, 0, 60, 20) {
		t.Fatalf("first label %v", got)
	}
	if i, plus := s.Hit(65); i != 1 || plus {
		t.Fatalf("x 65 hits %d %v, want tab 1", i, plus)
	}
	if _, plus := s.Hit(s.Plus.Min.X + 1); !plus {
		t.Fatal("the new tab button is not hit")
	}
	// Too many labels for the width share it, and the button stays inside.
	ts = named("one", "two", "three", "four", "five", "six")
	s = LayoutStrip(ts.Labels(), 10, 200, 20)
	if s.Plus.Max.X > 200 {
		t.Fatalf("the new tab button is off the strip at %d", s.Plus.Max.X)
	}
	if TitleOf(Tab{}, "/usr/bin/zsh") != "zsh" || TitleOf(Tab{Title: "visor"}, "x") != "visor" {
		t.Fatal("titles")
	}
}

func TestWindowSize(t *testing.T) {
	g := &Grid{CellW: 8, CellH: 16}
	strip := 16 + 16/3
	if got, want := WindowSize(g, 100, 30, image.Point{}), image.Pt(800, 30*16+strip); got != want {
		t.Errorf("unknown screen: %v, want %v", got, want)
	}
	if got := WindowSize(g, 100, 30, image.Pt(640, 480)); got.X > 576 || got.Y > 408 || got.Y >= got.X {
		t.Errorf("small screen: %v", got)
	}
	// A tall font on a tall screen still opens wider than tall.
	tall := &Grid{CellW: 8, CellH: 40}
	if got := WindowSize(tall, 30, 30, image.Pt(1080, 1920)); got.Y >= got.X {
		t.Errorf("portrait: %v", got)
	}
}
