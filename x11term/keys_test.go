//go:build unix && !android && !ios

package x11term

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/0magnet/progterm/glterm"
)

// A small keyboard: keycodes from 8, two keysyms each plus AltGr's pair.
//
//	8 a A  9 1 !  10 Shift_L  11 Control_L  12 Alt_L  13 Num_Lock
//	14 KP_Left KP_4  15 ISO_Level3_Shift  16 e E € (AltGr)  17 Up  18 F5
//	19 Tab ISO_Left_Tab  20 Cyrillic_a Cyrillic_A
func testKeymap() *keymap {
	const per = 6
	rows := [][per]uint32{
		{'a', 'A'}, {'1', '!'}, {0xffe1}, {0xffe3}, {xkAltL, xkMetaL}, {xkNumLock},
		{xkKPLeft, 0xffb4}, {xkISOLevel3Shift}, {'e', 'E', 0, 0, 0x20ac}, {xkUp}, {xkF1 + 4},
		{xkTab, xkISOLeftTab}, {0x6c1, 0x6e1},
	}
	var syms []uint32
	for _, r := range rows {
		syms = append(syms, r[:]...)
	}
	// Shift, Lock, Control, Mod1 (Alt), Mod2 (Num Lock), Mod3, Mod4, Mod5 (AltGr).
	return newKeymap(8, per, syms, 1, []byte{10, 0, 11, 12, 13, 0, 0, 15})
}

func TestKeymapModifiers(t *testing.T) {
	k := testKeymap()
	if k.alt != 1<<3 || k.numLock != 1<<4 || k.level3 != 1<<7 {
		t.Fatalf("alt %#x num lock %#x level3 %#x", k.alt, k.numLock, k.level3)
	}
	if m := k.mods(maskShift | maskControl | 1<<3); m != glterm.Shift|glterm.Ctrl|glterm.Alt {
		t.Fatalf("mods %v", m)
	}
}

func TestLookup(t *testing.T) {
	k := testKeymap()
	for _, c := range []struct {
		code  byte
		state uint16
		want  uint32
	}{
		{8, 0, 'a'},
		{8, maskShift, 'A'},
		{8, maskLock, 'A'},
		{9, maskLock, '1'}, // Caps Lock leaves what is not a letter
		{9, maskShift, '!'},
		{14, 0, xkKPLeft},
		{14, 1 << 4, 0xffb4},             // Num Lock: the digit
		{14, 1<<4 | maskShift, xkKPLeft}, // and Shift undoes it
		{16, 1 << 7, 0x20ac},             // AltGr
		{16, 1<<7 | maskShift, 0x20ac},   // with no fourth level, Shift adds nothing to it
		{19, maskShift, xkISOLeftTab},
		{20, maskShift, 0x6e1},
		{99, 0, 0},
	} {
		if got := k.lookup(c.code, c.state); got != c.want {
			t.Errorf("keycode %d state %#x: %#x, want %#x", c.code, c.state, got, c.want)
		}
	}
}

// bytesOf is what a key press sends to the program.
func bytesOf(ks uint32, m glterm.Mods) string {
	e, text, ok := translate(ks, m)
	if !ok {
		return "<none>"
	}
	if text != "" {
		return text
	}
	return glterm.Encode(e, false)
}

func TestTranslate(t *testing.T) {
	for _, c := range []struct {
		ks   uint32
		m    glterm.Mods
		want string
	}{
		{'a', 0, "a"},
		{'A', glterm.Shift, "A"},
		{0xe9, 0, "é"},
		{0x20ac, 0, "€"},
		{0x6c1, 0, "а"}, // Cyrillic_a
		{0x1000439, 0, "й"},
		{'c', glterm.Ctrl, "\x03"},
		{'C', glterm.Ctrl | glterm.Shift, "\x03"},
		{' ', glterm.Ctrl, "\x00"},
		{'[', glterm.Ctrl, "\x1b"},
		{'b', glterm.Alt, "\x1bb"},
		{'x', glterm.Ctrl | glterm.Alt, "\x1b\x18"},
		{xkReturn, 0, "\r"},
		{xkKPEnter, 0, "\r"},
		{xkBackSpace, 0, "\x7f"},
		{xkTab, 0, "\t"},
		{xkISOLeftTab, 0, "\x1b[Z"},
		{xkEscape, 0, "\x1b"},
		{xkUp, 0, "\x1b[A"},
		{xkLeft, glterm.Ctrl, "\x1b[1;5D"},
		{xkRight, glterm.Shift, "\x1b[1;2C"},
		{xkHome, 0, "\x1b[H"},
		{xkEnd, 0, "\x1b[F"},
		{xkPrior, 0, "\x1b[5~"},
		{xkNext, 0, "\x1b[6~"},
		{xkDelete, 0, "\x1b[3~"},
		{xkKPDelete, 0, "\x1b[3~"},
		{xkInsert, 0, "\x1b[2~"},
		{xkF1, 0, "\x1bOP"},
		{xkF1 + 4, 0, "\x1b[15~"},
		{xkF12, 0, "\x1b[24~"},
		{xkF1 + 4, glterm.Ctrl, "\x1b[15;5~"},
		{0xffb5, 0, "5"}, // KP_5
		{xkKPAdd, 0, "+"},
		{0xffe1, glterm.Shift, "<none>"}, // Shift_L on its own
		{0xfe51, 0, "<none>"},            // dead_acute: no compose here
	} {
		if got := bytesOf(c.ks, c.m); got != c.want {
			t.Errorf("keysym %#x mods %v: %q, want %q", c.ks, c.m, got, c.want)
		}
	}
}

func TestTranslateShortcuts(t *testing.T) {
	// The window's own shortcuts arrive as key events, not text.
	for _, c := range []struct {
		ks uint32
		m  glterm.Mods
	}{
		{'T', glterm.Ctrl | glterm.Shift}, {'V', glterm.Ctrl | glterm.Shift}, {'2', glterm.Alt}, {xkTab, glterm.Ctrl},
	} {
		e, text, ok := translate(c.ks, c.m)
		if !ok || text != "" || e.Mods != c.m {
			t.Errorf("keysym %#x: %+v %q %v", c.ks, e, text, ok)
		}
	}
	ts := &glterm.Tabs{}
	e, _, _ := translate('T', glterm.Ctrl|glterm.Shift)
	opened := false
	if !ts.Shortcut(e, func() { opened = true }) || !opened {
		t.Fatal("Ctrl+Shift+T opened no tab")
	}
}

func TestParseDPI(t *testing.T) {
	if got := parseDPI("Xft.antialias:\t1\nXft.dpi:\t97\nXft.hinting:\t1\n"); got != 97 {
		t.Fatalf("dpi %v", got)
	}
	if got := parseDPI("*background: black\n"); got != 0 {
		t.Fatalf("no dpi read as %v", got)
	}
}

func TestAnswer(t *testing.T) {
	at := atoms{targets: 100, utf8: 101, text: 102}
	const limit = 1000
	typ, format, data, ok := answer(at, at.targets, "x", true, limit)
	if !ok || typ != xproto.AtomAtom || format != 32 || len(data) != 16 {
		t.Fatalf("TARGETS: %v %d %d %v", typ, format, len(data), ok)
	}
	if typ, _, data, ok := answer(at, at.utf8, "héllo €", true, limit); !ok || typ != at.utf8 || string(data) != "héllo €" {
		t.Fatalf("UTF8_STRING: %v %q %v", typ, data, ok)
	}
	if typ, _, data, ok := answer(at, at.text, "a", true, limit); !ok || typ != at.utf8 || string(data) != "a" {
		t.Fatalf("TEXT: %v %q %v", typ, data, ok)
	}
	if typ, _, data, ok := answer(at, xproto.AtomString, "héllo €", true, limit); !ok || typ != xproto.AtomString || string(data) != "h\xe9llo ?" {
		t.Fatalf("STRING: %v %q %v", typ, data, ok)
	}
	if _, _, _, ok := answer(at, at.utf8, "x", false, limit); ok {
		t.Fatal("answered for a selection not held")
	}
	if _, _, _, ok := answer(at, 999, "x", true, limit); ok {
		t.Fatal("answered for an unknown target")
	}
	if _, _, _, ok := answer(at, at.utf8, string(make([]byte, limit+1)), true, limit); ok {
		t.Fatal("answered with more than one request holds")
	}
	if fromLatin1([]byte("h\xe9")) != "hé" {
		t.Fatal("latin1")
	}
}
