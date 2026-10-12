//go:build cgo && unix

// Package gioterm is glterm's terminal in a Gio window: the same session,
// grid, selection and menu, with Gio as the platform layer underneath —
// window, input, clipboard and the on-screen keyboard.
package gioterm

import (
	"fmt"
	"image"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/0magnet/progterm/glterm"
)

// Run opens a Gio window running opt.Shell, and ends the process when the
// shell exits or the window closes: Gio's app.Main owns the main goroutine
// and never returns, so the window runs on another.
func Run(opt glterm.Options) error {
	if opt.FontPt == 0 {
		opt.FontPt = 13
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("progterm"), app.Size(unit.Dp(600), unit.Dp(800)))
		if err := loop(w, opt); err != nil {
			fmt.Fprintln(os.Stderr, "progterm:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}

// clipboardBridge is glterm's Clipboard on Gio: reads and writes are commands
// a frame executes, so they are queued here and run at the next one. Gio has
// no primary selection; it is kept in the process.
type clipboardBridge struct {
	primary string
	write   *string
	read    func(string)
	w       *app.Window
}

func (c *clipboardBridge) Set(primary bool, text string) {
	if primary {
		c.primary = text
		return
	}
	c.write = &text
	c.w.Invalidate()
}

func (c *clipboardBridge) Get(primary bool, paste func(string)) {
	if primary {
		if c.primary != "" {
			paste(c.primary)
		}
		return
	}
	c.read = paste
	c.w.Invalidate()
}

func loop(w *app.Window, opt glterm.Options) error {
	var (
		ops     op.Ops
		tag     = new(int) // the terminal's input target
		t       = &glterm.Terminal{}
		cb      = &clipboardBridge{w: w}
		bufs    [2]*image.RGBA
		frame   int
		pxPerSp float32
		buttons pointer.Buttons // held before the last pointer event
		focused bool
		exited  = make(chan struct{})
	)
	t.Clip = cb
	defer func() {
		if t.Sess != nil {
			t.Sess.Close() //nolint:errcheck,gosec // the window is going
		}
	}()

	for {
		select {
		case <-exited:
			return nil
		default:
		}
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			size := e.Size

			if t.Grid == nil || e.Metric.PxPerSp != pxPerSp {
				g, err := glterm.NewGrid(opt.FontPt * float64(e.Metric.PxPerSp))
				if err != nil {
					return err
				}
				t.Grid, pxPerSp = g, e.Metric.PxPerSp
			}
			cols, rows := t.Grid.Size(size.X, size.Y)
			if t.Sess == nil {
				s, err := glterm.Start(opt.Shell, opt.Args, cols, rows)
				if err != nil {
					return fmt.Errorf("gioterm: %w", err)
				}
				t.Sess = s
				s.OnChange = w.Invalidate
				go func() {
					<-s.Done()
					close(exited)
					w.Invalidate()
				}()
			} else {
				t.Sess.Resize(cols, rows, size.X, size.Y)
			}
			if bufs[0] == nil || bufs[0].Bounds().Size() != size {
				bufs = [2]*image.RGBA{image.NewRGBA(image.Rectangle{Max: size}), image.NewRGBA(image.Rectangle{Max: size})}
				t.Resize(size.X, size.Y)
				t.Menu.Close()
			}

			// The whole window is the terminal's input target.
			area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
			event.Op(gtx.Ops, tag)
			if !focused {
				gtx.Execute(key.FocusCmd{Tag: tag})
				focused = true
			}
			handle(gtx.Source, tag, t, cb, &buttons)
			area.Pop()

			if cb.write != nil {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(*cb.write))})
				cb.write = nil
			}
			if cb.read != nil {
				gtx.Execute(clipboard.ReadCmd{Tag: tag})
			}

			img := bufs[frame%2]
			frame++
			t.Draw(img)
			paint.NewImageOp(img).Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			e.Frame(gtx.Ops)
		}
	}
}

// handle reads the frame's input and gives it to the terminal.
func handle(src input.Source, tag *int, t *glterm.Terminal, cb *clipboardBridge, held *pointer.Buttons) {
	filters := []event.Filter{
		key.FocusFilter{Target: tag},
		// Keys that are not text: named keys, and anything with Ctrl or Alt.
		// Plain typing comes as EditEvents, which an on-screen keyboard and an
		// input method also send.
		key.Filter{Focus: tag, Name: key.NameTab, Optional: key.ModShift},
		key.Filter{Focus: tag, Name: "", Required: key.ModCtrl, Optional: key.ModShift | key.ModAlt},
		key.Filter{Focus: tag, Name: "", Required: key.ModAlt, Optional: key.ModShift},
		pointer.Filter{
			Target:  tag,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		},
		transfer.TargetFilter{Target: tag, Type: "application/text"},
	}
	for _, n := range []key.Name{key.NameReturn, key.NameEnter, key.NameEscape, key.NameDeleteBackward,
		key.NameDeleteForward, key.NameUpArrow, key.NameDownArrow, key.NameLeftArrow, key.NameRightArrow,
		key.NameHome, key.NameEnd, key.NamePageUp, key.NamePageDown,
		key.NameF1, key.NameF2, key.NameF3, key.NameF4, key.NameF5, key.NameF6,
		key.NameF7, key.NameF8, key.NameF9, key.NameF10, key.NameF11, key.NameF12} {
		filters = append(filters, key.Filter{Focus: tag, Name: n, Optional: key.ModShift | key.ModCtrl | key.ModAlt})
	}
	for {
		ev, ok := src.Event(filters...)
		if !ok {
			return
		}
		switch ev := ev.(type) {
		case key.EditEvent:
			t.Text(ev.Text)
		case key.Event:
			if ev.State != key.Press {
				continue
			}
			if ke, ok := termKey(ev); ok {
				t.Key(ke)
			}
		case pointer.Event:
			if ev.Kind == pointer.Press {
				src.Execute(key.FocusCmd{Tag: tag})
				src.Execute(key.SoftKeyboardCmd{Show: true})
			}
			for _, me := range termMouse(ev, held) {
				t.Mouse(me)
			}
		case transfer.DataEvent:
			if paste := cb.read; paste != nil {
				cb.read = nil
				r := ev.Open()
				b, err := io.ReadAll(r)
				r.Close() //nolint:errcheck,gosec // read in full above
				if err == nil && len(b) > 0 {
					paste(string(b))
				}
			}
		}
	}
}

// names maps Gio's key names to glterm's.
var names = map[key.Name]glterm.Key{
	key.NameReturn: glterm.KeyEnter, key.NameEnter: glterm.KeyEnter, key.NameEscape: glterm.KeyEscape,
	key.NameDeleteBackward: glterm.KeyBackspace, key.NameDeleteForward: glterm.KeyDelete,
	key.NameTab: glterm.KeyTab, key.NameUpArrow: glterm.KeyUp, key.NameDownArrow: glterm.KeyDown,
	key.NameLeftArrow: glterm.KeyLeft, key.NameRightArrow: glterm.KeyRight,
	key.NameHome: glterm.KeyHome, key.NameEnd: glterm.KeyEnd, key.NamePageUp: glterm.KeyPageUp, key.NamePageDown: glterm.KeyPageDown,
	key.NameF1: glterm.KeyF1, key.NameF2: glterm.KeyF2, key.NameF3: glterm.KeyF3, key.NameF4: glterm.KeyF4,
	key.NameF5: glterm.KeyF5, key.NameF6: glterm.KeyF6, key.NameF7: glterm.KeyF7, key.NameF8: glterm.KeyF8,
	key.NameF9: glterm.KeyF9, key.NameF10: glterm.KeyF10, key.NameF11: glterm.KeyF11, key.NameF12: glterm.KeyF12,
}

// mods is Gio's modifiers as glterm's.
func mods(m key.Modifiers) glterm.Mods {
	var g glterm.Mods
	if m.Contain(key.ModShift) {
		g |= glterm.Shift
	}
	if m.Contain(key.ModCtrl) {
		g |= glterm.Ctrl
	}
	if m.Contain(key.ModAlt) {
		g |= glterm.Alt
	}
	if m.Contain(key.ModSuper) {
		g |= glterm.Meta
	}
	return g
}

// termKey is a Gio key press as glterm's KeyEvent.
func termKey(e key.Event) (glterm.KeyEvent, bool) {
	ke := glterm.KeyEvent{Mods: mods(e.Modifiers)}
	if e.Name == key.NameSpace {
		ke.Rune = ' '
		return ke, true
	}
	if k, ok := names[e.Name]; ok {
		ke.Key = k
		return ke, true
	}
	// A letter, digit or symbol held with Ctrl or Alt: Gio names it by the
	// character, upper case for letters.
	r, n := utf8.DecodeRuneInString(string(e.Name))
	if n == 0 || n != len(e.Name) {
		return ke, false
	}
	if r >= 'A' && r <= 'Z' && !e.Modifiers.Contain(key.ModShift) {
		r += 'a' - 'A'
	}
	ke.Rune = r
	return ke, true
}

// termMouse is a Gio pointer event as glterm's MouseEvents. Gio reports which
// buttons are held; a press or release is of the ones that changed.
func termMouse(e pointer.Event, held *pointer.Buttons) []glterm.MouseEvent {
	at := func(b glterm.Button, a glterm.Action) glterm.MouseEvent {
		return glterm.MouseEvent{X: e.Position.X, Y: e.Position.Y, Button: b, Action: a, Mods: mods(e.Modifiers)}
	}
	var out []glterm.MouseEvent
	switch e.Kind {
	case pointer.Press, pointer.Release:
		changed := e.Buttons ^ *held
		a := glterm.Press
		if e.Kind == pointer.Release {
			a = glterm.Release
		}
		for _, b := range []struct {
			g pointer.Buttons
			t glterm.Button
		}{{pointer.ButtonPrimary, glterm.ButtonLeft}, {pointer.ButtonSecondary, glterm.ButtonRight}, {pointer.ButtonTertiary, glterm.ButtonMiddle}} {
			if changed&b.g != 0 {
				out = append(out, at(b.t, a))
			}
		}
		if e.Source == pointer.Touch && len(out) == 0 {
			out = append(out, at(glterm.ButtonLeft, a))
		}
	case pointer.Drag, pointer.Move:
		out = append(out, at(glterm.ButtonNone, glterm.Move))
	case pointer.Scroll:
		if e.Scroll != (f32.Point{}) {
			b := glterm.WheelDown
			if e.Scroll.Y < 0 {
				b = glterm.WheelUp
			}
			out = append(out, at(b, glterm.Press))
		}
	}
	*held = e.Buttons
	return out
}
