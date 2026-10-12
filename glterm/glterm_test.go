package glterm

import (
	"image"
	"image/color"
	"testing"

	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/xterm-go/vt"
)

// render draws s on a cols by rows terminal and returns the picture.
func render(t *testing.T, cols, rows int, s string) (*Grid, *image.RGBA) {
	t.Helper()
	g, err := NewGrid(16)
	if err != nil {
		t.Fatal(err)
	}
	term := vt.NewTerminal(nil)
	term.Resize(cols, rows)
	term.WriteString(s)
	img := image.NewRGBA(image.Rect(0, 0, cols*g.CellW, rows*g.CellH))
	g.Draw(term, img)
	return g, img
}

// ink is how many pixels of cell col, on the first row, are not the background, and the
// color most of them are.
func ink(g *Grid, img *image.RGBA, col int) (n int, c color.RGBA) {
	counts := map[color.RGBA]int{}
	for y := 0; y < g.CellH; y++ {
		for x := col * g.CellW; x < (col+1)*g.CellW; x++ {
			p := img.RGBAAt(x, y)
			if p != g.bg {
				n++
				counts[p]++
			}
		}
	}
	for k, v := range counts {
		if v > counts[c] {
			c = k
		}
	}
	return n, c
}

func TestGridDrawsColoredText(t *testing.T) {
	g, img := render(t, 10, 2, "\x1b[31mA\x1b[0m \x1b[42m \x1b[0m")
	if n, c := ink(g, img, 0); n == 0 || c.R < 0x80 || c.G > 0x40 {
		t.Errorf("cell 0: %d pixels, mostly %v; want red ink", n, c)
	}
	if n, _ := ink(g, img, 1); n != 0 {
		t.Errorf("cell 1 is a space, has %d pixels", n)
	}
	if n, c := ink(g, img, 2); n != g.CellW*g.CellH || c.G < 0x80 {
		t.Errorf("cell 2: %d pixels of %v; want it filled green", n, c)
	}
}

func TestGridDrawsTheCursor(t *testing.T) {
	g, img := render(t, 10, 2, "ab")
	if n, _ := ink(g, img, 2); n != g.CellW*g.CellH {
		t.Errorf("the cursor's cell has %d pixels of %d", n, g.CellW*g.CellH)
	}
	g, img = render(t, 10, 2, "ab\x1b[?25l")
	if n, _ := ink(g, img, 2); n != 0 {
		t.Errorf("a hidden cursor drew %d pixels", n)
	}
}

func TestGridDrawsBraille(t *testing.T) {
	g, img := render(t, 4, 1, "⣿⠀\x1b[?25l")
	if n, _ := ink(g, img, 0); n == 0 {
		t.Error("a full braille cell drew nothing")
	}
	if n, _ := ink(g, img, 1); n != 0 {
		t.Errorf("an empty braille cell drew %d pixels", n)
	}
}

func TestEncode(t *testing.T) {
	press := func(r rune, c key.Code, m key.Modifiers) key.Event {
		return key.Event{Rune: r, Code: c, Modifiers: m, Direction: key.DirPress}
	}
	for _, tc := range []struct {
		e         key.Event
		appCursor bool
		want      string
	}{
		{press('a', key.CodeA, 0), false, "a"},
		{press('A', key.CodeA, key.ModShift), false, "A"},
		{press('c', key.CodeC, key.ModControl), false, "\x03"},
		{press('x', key.CodeX, key.ModAlt), false, "\x1bx"},
		{press('[', key.CodeLeftSquareBracket, key.ModControl), false, "\x1b"},
		{press('α', 0, 0), false, "α"},
		{press(-1, key.CodeReturnEnter, 0), false, "\r"},
		{press(-1, key.CodeDeleteBackspace, 0), false, "\x7f"},
		{press(-1, key.CodeUpArrow, 0), false, "\x1b[A"},
		{press(-1, key.CodeUpArrow, 0), true, "\x1bOA"},
		{press(-1, key.CodeF5, 0), false, "\x1b[15~"},
		{press(-1, key.CodeLeftShift, key.ModShift), false, ""},
		{key.Event{Rune: 'a', Code: key.CodeA, Direction: key.DirRelease}, false, ""},
	} {
		if got := Encode(tc.e, tc.appCursor); got != tc.want {
			t.Errorf("Encode(%v, %v) = %q, want %q", tc.e, tc.appCursor, got, tc.want)
		}
	}
}
