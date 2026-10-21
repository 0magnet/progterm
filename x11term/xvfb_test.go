//go:build unix && !android && !ios

package x11term

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"

	"github.com/0magnet/progterm/glterm"
)

// xvfb starts an Xvfb of its own for the test and returns its display, or
// skips the test where there is no Xvfb.
func xvfb(t *testing.T) string {
	t.Helper()
	xgb.Logger.SetOutput(io.Discard)
	bin, err := exec.LookPath("Xvfb")
	if err != nil {
		t.Skip("no Xvfb")
	}
	for n := 140; n < 200; n++ {
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", n)); err == nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X11-unix/X%d", n)); err == nil {
			continue
		}
		disp := fmt.Sprintf(":%d", n)
		cmd := exec.Command(bin, disp, "-screen", "0", "1280x800x24", "-nolisten", "tcp", "-noreset") //nolint:gosec // our own display
		if err := cmd.Start(); err != nil {
			t.Skipf("Xvfb: %v", err)
		}
		t.Cleanup(func() {
			cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck,gosec // ours; it removes its lock and socket
			cmd.Wait()                          //nolint:errcheck,gosec // killed
		})
		for i := 0; i < 100; i++ {
			if c, err := xgb.NewConnDisplay(disp); err == nil {
				c.Close()
				return disp
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("Xvfb %s did not come up", disp)
	}
	t.Skip("no free display")
	return ""
}

// on runs f on w's goroutine and waits for it.
func on[T any](w *window, f func() T) T {
	ch := make(chan T, 1)
	w.do(func() { ch <- f() })
	return <-ch
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// screenRow is row of the current tab's screen as text.
func screenRow(w *window, row int) string {
	return on(w, func() string {
		t := w.ts.Active()
		if t == nil {
			return ""
		}
		s := t.Term.Sess
		s.Lock()
		defer s.Unlock()
		buf := s.Term.Buffer()
		line := buf.Lines.Get(buf.YBase + row)
		if line == nil {
			return ""
		}
		return line.TranslateToString(true, 0, s.Term.Cols())
	})
}

// other is a second X client: it types with XTEST and holds and asks for
// selections as another program would.
type other struct {
	t        *testing.T
	c        *xgb.Conn
	root     xproto.Window
	win      xproto.Window
	keys     *keymap
	at       atoms
	owns     string
	notified chan xproto.SelectionNotifyEvent
}

func newOther(t *testing.T, disp string) *other {
	c, err := xgb.NewConnDisplay(disp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	if err := xtest.Init(c); err != nil {
		t.Skipf("no XTEST: %v", err)
	}
	o := &other{t: t, c: c, notified: make(chan xproto.SelectionNotifyEvent, 4)}
	setup := xproto.Setup(c)
	o.root = setup.DefaultScreen(c).Root
	w := &window{conn: c, setup: setup}
	if err := w.loadKeymap(); err != nil {
		t.Fatal(err)
	}
	o.keys = w.keys
	if o.at, err = internAtoms(c); err != nil {
		t.Fatal(err)
	}
	if o.win, err = xproto.NewWindowId(c); err != nil {
		t.Fatal(err)
	}
	xproto.CreateWindow(c, 0, o.win, o.root, 0, 0, 1, 1, 0, xproto.WindowClassInputOnly, 0, 0, nil)
	go func() {
		for {
			ev, _ := c.WaitForEvent() //nolint:errcheck // errors are seen as failed steps
			switch e := ev.(type) {
			case nil:
				return
			case xproto.SelectionNotifyEvent:
				o.notified <- e
			case xproto.SelectionRequestEvent:
				_, format, data, ok := answer(o.at, e.Target, o.owns, true, 1<<16)
				prop := xproto.Atom(xproto.AtomNone)
				if ok {
					prop = e.Property
					typ := o.at.utf8
					if e.Target == o.at.targets {
						typ = xproto.AtomAtom
					}
					xproto.ChangeProperty(c, xproto.PropModeReplace, e.Requestor, prop, typ, format, uint32(len(data)*8/int(format)), data) //nolint:gosec // small
				}
				ev := xproto.SelectionNotifyEvent{Time: e.Time, Requestor: e.Requestor, Selection: e.Selection, Target: e.Target, Property: prop}
				xproto.SendEvent(c, false, e.Requestor, 0, string(ev.Bytes()))
			}
		}
	}()
	return o
}

func (o *other) code(ks uint32) byte {
	for code := int(o.keys.min); code < int(o.keys.min)+len(o.keys.syms)/o.keys.per; code++ {
		if o.keys.sym(byte(code), 0) == ks { //nolint:gosec // keycodes are 8 to 255
			return byte(code) //nolint:gosec // as above
		}
	}
	o.t.Fatalf("no keycode for keysym %#x", ks)
	return 0
}

// press presses and releases the keys of keysyms ks, in order and then in
// reverse, as a chord.
func (o *other) press(ks ...uint32) {
	for _, k := range ks {
		xtest.FakeInputChecked(o.c, xproto.KeyPress, o.code(k), 0, o.root, 0, 0, 0).Check() //nolint:errcheck,gosec // seen on screen
	}
	for i := len(ks) - 1; i >= 0; i-- {
		xtest.FakeInputChecked(o.c, xproto.KeyRelease, o.code(ks[i]), 0, o.root, 0, 0, 0).Check() //nolint:errcheck,gosec // as above
	}
}

func (o *other) typeText(s string) {
	for _, r := range s {
		o.press(uint32(r)) //nolint:gosec // a code point
	}
}

const (
	xkShiftL   = 0xffe1
	xkControlL = 0xffe3
)

func TestXvfb(t *testing.T) {
	disp := xvfb(t)
	o := newOther(t, disp)
	res := "Xft.dpi:\t120\n"
	xproto.ChangePropertyChecked(o.c, xproto.PropModeReplace, o.root, xproto.AtomResourceManager, xproto.AtomString, 8, uint32(len(res)), []byte(res)).Check() //nolint:errcheck,gosec // checked by the cell size below

	w, err := open(glterm.Options{Shell: "/bin/cat", Font: "Go Mono 10", Title: "x11term-test"}, disp)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := glterm.NewGridFont(glterm.GoMono(10), 10*120.0/72) //nolint:errcheck // Go Mono loads
	if w.grid.CellH != want.CellH || w.grid.CellW != want.CellW {
		t.Fatalf("cells %dx%d at Xft.dpi 120, want %dx%d", w.grid.CellW, w.grid.CellH, want.CellW, want.CellH)
	}
	done := make(chan error, 1)
	go func() { done <- w.run() }()
	eventually(t, "the first tab", func() bool { return on(w, func() int { return len(w.ts.List) }) == 1 })
	xproto.SetInputFocusChecked(o.c, xproto.InputFocusParent, w.win, xproto.TimeCurrentTime).Check() //nolint:errcheck,gosec // seen below

	o.typeText("hi")
	eventually(t, "typed text", func() bool { return strings.HasPrefix(screenRow(w, 0), "hi") })

	// The text reached the window, not just the terminal.
	eventually(t, "drawn text", func() bool {
		img, err := xproto.GetImage(o.c, xproto.ImageFormatZPixmap, xproto.Drawable(w.win),
			0, int16(w.stripH), uint16(2*w.grid.CellW), uint16(w.grid.CellH), 0xffffffff).Reply() //nolint:gosec // small
		if err != nil {
			return false
		}
		for _, b := range img.Data {
			if b != 0 {
				return true
			}
		}
		return false
	})

	// Ctrl+= and Ctrl+- scale the font, the terminal following in rows, and
	// Ctrl+0 puts it back.
	cellH := func() int { return on(w, func() int { return w.grid.CellH }) }
	rows := func() int { return on(w, func() int { return w.ts.Active().Term.Sess.Term.Rows() }) }
	h0, r0 := cellH(), rows()
	o.press(xkControlL, '=')
	eventually(t, "a larger font", func() bool { return cellH() > h0 && rows() < r0 })
	o.press(xkControlL, '-')
	o.press(xkControlL, '-')
	eventually(t, "a smaller font", func() bool { return cellH() < h0 && rows() > r0 })
	o.press(xkControlL, '0')
	eventually(t, "the font back", func() bool { return cellH() == h0 && rows() == r0 })

	// The terminal's copy, pasted elsewhere.
	on(w, func() bool { w.clip.Set(false, "from-term"); return true })
	xproto.ConvertSelection(o.c, o.win, o.at.clipboard, o.at.utf8, o.at.prop, xproto.TimeCurrentTime)
	select {
	case e := <-o.notified:
		if e.Property == xproto.AtomNone {
			t.Fatal("the terminal refused its own clipboard")
		}
		r, err := xproto.GetProperty(o.c, true, o.win, e.Property, xproto.GetPropertyTypeAny, 0, 1<<16).Reply()
		if err != nil || string(r.Value) != "from-term" {
			t.Fatalf("clipboard %q, %v", r.Value, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no answer for the clipboard")
	}

	// Another program's copy, pasted in the terminal with Ctrl+Shift+V.
	o.owns = " pasted"
	xproto.SetSelectionOwnerChecked(o.c, o.win, o.at.clipboard, xproto.TimeCurrentTime).Check() //nolint:errcheck,gosec // seen below
	o.press(xkControlL, xkShiftL, 'v')
	eventually(t, "pasted text", func() bool { return strings.HasPrefix(screenRow(w, 0), "hi pasted") })

	// Middle click pastes the primary selection, held here by a selection.
	on(w, func() bool { w.clip.Set(true, "!"); return true })
	y := int16(w.stripH + 10)                                         //nolint:gosec // small
	xtest.FakeInput(o.c, xproto.MotionNotify, 0, 0, o.root, 10, y, 0) // the window is at 0, 0 with no window manager
	xtest.FakeInput(o.c, xproto.ButtonPress, 2, 0, o.root, 10, y, 0)
	xtest.FakeInput(o.c, xproto.ButtonRelease, 2, 0, o.root, 10, y, 0)
	eventually(t, "middle click paste", func() bool { return strings.HasPrefix(screenRow(w, 0), "hi pasted!") })

	o.press(xkControlL, xkShiftL, 't')
	eventually(t, "a second tab", func() bool { return on(w, func() int { return len(w.ts.List) }) == 2 })
	o.press(xkControlL, xkTab)
	eventually(t, "back to the first tab", func() bool { return on(w, func() int { return w.ts.Cur }) == 0 })

	// A resize reaches the pty.
	size := []uint32{uint32(40 * w.grid.CellW), uint32(w.stripH + 10*w.grid.CellH)}                             //nolint:gosec // small
	xproto.ConfigureWindowChecked(o.c, w.win, xproto.ConfigWindowWidth|xproto.ConfigWindowHeight, size).Check() //nolint:errcheck,gosec // seen below
	eventually(t, "a resize", func() bool {
		return on(w, func() bool {
			term := w.ts.Active().Term.Sess.Term
			return term.Cols() == 40 && term.Rows() == 10
		})
	})

	// The window manager's close button.
	msg := xproto.ClientMessageEvent{Format: 32, Window: w.win, Type: w.at.wmProtocols,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{uint32(w.at.wmDelete), 0, 0, 0, 0})}
	xproto.SendEvent(o.c, false, w.win, 0, string(msg.Bytes()))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the window did not close")
	}
}
