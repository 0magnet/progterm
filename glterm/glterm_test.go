package glterm

import (
	"image"
	"image/color"
	"testing"

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
	g.Draw(term, img, nil)
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
	for _, tc := range []struct {
		e         KeyEvent
		appCursor bool
		want      string
	}{
		{KeyEvent{Rune: 'a'}, false, "a"},
		{KeyEvent{Rune: 'c', Mods: Ctrl}, false, "\x03"},
		{KeyEvent{Rune: 'x', Mods: Alt}, false, "\x1bx"},
		{KeyEvent{Rune: '[', Mods: Ctrl}, false, "\x1b"},
		{KeyEvent{Rune: 'α'}, false, "α"},
		{KeyEvent{Key: KeyEnter}, false, "\r"},
		{KeyEvent{Key: KeyBackspace}, false, "\x7f"},
		{KeyEvent{Key: KeyUp}, false, "\x1b[A"},
		{KeyEvent{Key: KeyUp}, true, "\x1bOA"},
		{KeyEvent{Key: KeyF5}, false, "\x1b[15~"},
		{KeyEvent{Key: KeyTab, Mods: Shift}, false, "\x1b[Z"},
		{KeyEvent{}, false, ""},
	} {
		if got := Encode(tc.e, tc.appCursor); got != tc.want {
			t.Errorf("Encode(%+v, %v) = %q, want %q", tc.e, tc.appCursor, got, tc.want)
		}
	}
}

func TestSelectionText(t *testing.T) {
	term := vt.NewTerminal(nil)
	term.Resize(10, 4)
	term.WriteString("hello wor\r\nld two\r\nthree")
	var s Selection
	s.Start(Pos{0, 6}, ByChar)
	s.Extend(Pos{1, 1})
	if got := s.Text(term); got != "wor\nld" {
		t.Errorf("drag: %q", got)
	}
	s.Start(Pos{1, 4}, ByWord)
	if got := s.Text(term); got != "two" {
		t.Errorf("word: %q", got)
	}
	s.Start(Pos{2, 2}, ByLine)
	if got := s.Text(term); got != "three" {
		t.Errorf("line: %q", got)
	}
	s.Start(Pos{0, 3}, ByChar)
	if !s.Empty() || s.Text(term) != "" {
		t.Error("a click alone selected something")
	}
}

// A line the terminal wrapped is one line of copied text.
func TestSelectionJoinsWrappedLines(t *testing.T) {
	term := vt.NewTerminal(nil)
	term.Resize(5, 4)
	term.WriteString("abcdefgh")
	var s Selection
	s.Start(Pos{0, 0}, ByChar)
	s.Extend(Pos{1, 2})
	if got := s.Text(term); got != "abcdefgh" {
		t.Errorf("got %q", got)
	}
}

func TestMenu(t *testing.T) {
	g, err := NewGrid(16)
	if err != nil {
		t.Fatal(err)
	}
	var ran string
	var m Menu
	m.Show(g, 10, 10, image.Rect(0, 0, 400, 400), []MenuItem{
		{"Copy", "", false, func() { ran = "copy" }},
		{"Paste", "", true, func() { ran = "paste" }},
	})
	row := func(i int) int { return 10 + m.rowH/4 + i*m.rowH + m.rowH/2 }
	if m.Click(20, row(0)); ran != "" || m.Open {
		t.Errorf("a disabled item ran (%q) or the menu stayed open", ran)
	}
	m.Show(g, 10, 10, image.Rect(0, 0, 400, 400), m.Items)
	if m.Click(20, row(1)); ran != "paste" {
		t.Errorf("clicking Paste ran %q", ran)
	}
	m.Show(g, 390, 390, image.Rect(0, 0, 400, 400), m.Items)
	if !m.rect.In(image.Rect(0, 0, 400, 400)) {
		t.Errorf("menu at the corner spills out: %v", m.rect)
	}
}
