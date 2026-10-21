package gioterm

import (
	"strings"
	"testing"

	"github.com/0magnet/gio/io/input"
	"github.com/0magnet/gio/io/key"
)

// TestIMEMirror replays what Gboard sent for "hello" and a correction, and
// checks the terminal ends up with the text the input method shows.
func TestIMEMirror(t *testing.T) {
	var m imeMirror
	var out []rune
	apply := func(start, end int, text string) {
		erase, typed := m.edit(key.EditEvent{Range: key.Range{Start: start, End: end}, Text: text})
		out = append(out[:len(out)-erase], []rune(typed)...)
	}
	for i, c := range "hello" {
		apply(i, i, string(c))
	}
	apply(0, 5, "Hello,") // a correction replaces the word
	apply(6, 6, " wörld")
	apply(8, 9, "o")  // an edit in the middle retypes the tail
	apply(11, 12, "") // the keyboard's delete
	if got, want := string(out), "Hello, worl"; got != want {
		t.Fatalf("terminal has %q, want %q", got, want)
	}
	if got := string(m.text); got != string(out) {
		t.Fatalf("mirror has %q, terminal %q", got, string(out))
	}
	apply(-1, 99, "x") // out of range clamps to the whole text
	if string(m.text) != "x" || string(out) != "x" {
		t.Fatalf("clamped edit gave mirror %q, terminal %q", string(m.text), string(out))
	}
	m.backspace()
	m.backspace()
	if len(m.text) != 0 {
		t.Fatalf("backspace left %q", string(m.text))
	}
}

func TestIMEMirrorReport(t *testing.T) {
	m := imeMirror{text: []rune("añb")}
	tag := new(int)
	var cmds []string
	m.report(func(c input.Command) {
		switch c := c.(type) {
		case key.SnippetCmd:
			cmds = append(cmds, "snippet "+c.Text)
			if c.Range != (key.Range{End: 3}) {
				t.Errorf("snippet range %v", c.Range)
			}
		case key.SelectionCmd:
			cmds = append(cmds, "selection")
			if c.Range != (key.Range{Start: 3, End: 3}) {
				t.Errorf("selection %v", c.Range)
			}
		}
	}, tag)
	if got := strings.Join(cmds, ","); got != "snippet añb,selection" {
		t.Fatalf("report sent %s", got)
	}
}

// TestIMEMirrorNewLine checks that ending a line keeps the text, so the
// input method is not restarted, and that nothing before the line is erased.
func TestIMEMirrorNewLine(t *testing.T) {
	var m imeMirror
	m.edit(key.EditEvent{Range: key.Range{}, Text: "ls"})
	m.newLine()
	if string(m.text) != "ls" || m.base != 2 {
		t.Fatalf("after a line: text %q, base %d", string(m.text), m.base)
	}
	erase, typed := m.edit(key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "pwd"})
	if erase != 0 || typed != "pwd" || string(m.text) != "lspwd" {
		t.Fatalf("an edit before the line erased %d, typed %q, left %q", erase, typed, string(m.text))
	}
	m.backspace()
	m.backspace()
	m.backspace()
	m.backspace()
	if string(m.text) != "ls" {
		t.Fatalf("backspace past the line left %q", string(m.text))
	}
	m.text = make([]rune, imeMax+1)
	m.newLine()
	if len(m.text) != 0 || m.base != 0 {
		t.Fatalf("a long text was kept: %d runes, base %d", len(m.text), m.base)
	}
}
