package glterm

import (
	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/xterm-go/vt"
)

// special are the keys without a character, as the browser names them: the
// key code and key name vt's encoder takes (it is xterm.js's).
var special = map[key.Code]struct {
	code int
	name string
}{
	key.CodeReturnEnter: {13, "Enter"}, key.CodeKeypadEnter: {13, "Enter"},
	key.CodeEscape: {27, "Escape"}, key.CodeDeleteBackspace: {8, "Backspace"}, key.CodeTab: {9, "Tab"},
	key.CodeInsert: {45, "Insert"}, key.CodeDeleteForward: {46, "Delete"},
	key.CodeHome: {36, "Home"}, key.CodeEnd: {35, "End"}, key.CodePageUp: {33, "PageUp"}, key.CodePageDown: {34, "PageDown"},
	key.CodeLeftArrow: {37, "ArrowLeft"}, key.CodeUpArrow: {38, "ArrowUp"},
	key.CodeRightArrow: {39, "ArrowRight"}, key.CodeDownArrow: {40, "ArrowDown"},
	key.CodeF1: {112, "F1"}, key.CodeF2: {113, "F2"}, key.CodeF3: {114, "F3"}, key.CodeF4: {115, "F4"},
	key.CodeF5: {116, "F5"}, key.CodeF6: {117, "F6"}, key.CodeF7: {118, "F7"}, key.CodeF8: {119, "F8"},
	key.CodeF9: {120, "F9"}, key.CodeF10: {121, "F10"}, key.CodeF11: {122, "F11"}, key.CodeF12: {123, "F12"},
}

// Encode is what a key press sends to the program: "" for a release, a
// modifier alone, or a key the terminal has no sequence for. appCursor is
// whether the program asked for application cursor keys (DECCKM).
func Encode(e key.Event, appCursor bool) string {
	if e.Direction == key.DirRelease {
		return ""
	}
	ctrl, alt, shift := e.Modifiers&key.ModControl != 0, e.Modifiers&key.ModAlt != 0, e.Modifiers&key.ModShift != 0
	if s, ok := special[e.Code]; ok {
		r := vt.EvaluateKeyboardEvent(&vt.KeyboardEvent{
			KeyCode: s.code, Key: s.name, CtrlKey: ctrl, AltKey: alt, ShiftKey: shift,
		}, appCursor, false, false)
		return r.Key
	}
	if e.Rune < 0 {
		return ""
	}
	r := e.Rune
	if ctrl {
		c, ok := control(r)
		if !ok {
			return ""
		}
		r = c
	}
	if alt {
		return "\x1b" + string(r) // Alt is Meta: ESC before the key
	}
	return string(r)
}

// control is Ctrl with r, as a terminal sends it.
func control(r rune) (rune, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return r - 'a' + 1, true
	case r >= '@' && r <= '_': // @ A-Z [ \ ] ^ _
		return r - '@', true
	case r == ' ' || r == '2':
		return 0, true
	case r == '3':
		return 0x1b, true
	case r == '4':
		return 0x1c, true
	case r == '5':
		return 0x1d, true
	case r == '6':
		return 0x1e, true
	case r == '7' || r == '-' || r == '/':
		return 0x1f, true
	case r == '8' || r == '?':
		return 0x7f, true
	}
	return 0, false
}
