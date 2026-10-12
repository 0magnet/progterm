//go:build cgo && unix

package glterm

import (
	"fmt"
	"sync/atomic"

	"github.com/0magnet/mobile/app"
	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/mobile/event/lifecycle"
	"github.com/0magnet/mobile/event/mouse"
	"github.com/0magnet/mobile/event/paint"
	"github.com/0magnet/mobile/event/size"
	"github.com/0magnet/mobile/exp/gl/glutil"
	"github.com/0magnet/mobile/geom"
	"github.com/0magnet/mobile/gl"
)

// exited is sent through the app's events when the shell has ended.
type exited struct{}

// systemClipboard is gomobile's clipboard. A read waits on the clipboard's
// owner, so it is made off the event loop and the paste follows when it
// comes.
type systemClipboard struct{}

func (systemClipboard) Set(primary bool, text string) {
	app.SetClipboard(primary, text) //nolint:errcheck,gosec // nowhere to report it; the next copy tries again
}

func (systemClipboard) Get(primary bool, paste func(string)) {
	go func() {
		if text, err := app.ReadClipboard(primary); err == nil && text != "" {
			paste(text)
		}
	}()
}

// Run opens the window and runs the shell in it until the shell exits or the
// window is closed. It must be called from the main goroutine (gomobile's
// app.Main).
func Run(opt Options) error {
	if opt.FontPt == 0 {
		opt.FontPt = 13
	}
	var runErr error
	app.Main(func(a app.App) {
		runErr = loop(a, opt)
	})
	return runErr
}

func loop(a app.App, opt Options) error {
	var (
		glctx  gl.Context
		images *glutil.Images
		img    *glutil.Image
		sz     size.Event
		ppp    float32
		t      = &Terminal{Clip: systemClipboard{}}
		dirty  atomic.Bool // a paint is wanted and not yet sent
	)
	defer func() {
		if t.Sess != nil {
			t.Sess.Close() //nolint:errcheck,gosec // the window is going
		}
	}()
	repaint := func() {
		if dirty.CompareAndSwap(false, true) {
			a.Send(paint.Event{})
		}
	}

	for e := range a.Events() {
		switch e := a.Filter(e).(type) {
		case lifecycle.Event:
			switch e.Crosses(lifecycle.StageVisible) {
			case lifecycle.CrossOn:
				glctx, _ = e.DrawContext.(gl.Context)
				images = glutil.NewImages(glctx)
				img = nil // made at the next size
				repaint()
			case lifecycle.CrossOff:
				if img != nil {
					img.Release()
					img = nil
				}
				if images != nil {
					images.Release()
					images = nil
				}
				glctx = nil
			}
			if e.To == lifecycle.StageDead {
				return nil
			}

		case size.Event:
			sz = e
			if t.Grid == nil || e.PixelsPerPt != ppp {
				g, err := NewGrid(opt.FontPt * float64(e.PixelsPerPt))
				if err != nil {
					return err
				}
				t.Grid, ppp = g, e.PixelsPerPt
			}
			t.Resize(e.WidthPx, e.HeightPx)
			t.Menu.Close()
			cols, rows := t.Grid.Size(e.WidthPx, e.HeightPx)
			if t.Sess == nil {
				s, err := Start(opt.Shell, opt.Args, cols, rows)
				if err != nil {
					return fmt.Errorf("glterm: %w", err)
				}
				t.Sess = s
				s.OnChange = repaint
				go func() {
					<-s.Done()
					a.Send(exited{})
				}()
			} else {
				t.Sess.Resize(cols, rows, e.WidthPx, e.HeightPx)
			}
			if img != nil {
				img.Release()
				img = nil
			}
			repaint()

		case paint.Event:
			dirty.Store(false)
			if glctx == nil || t.Sess == nil || sz.WidthPx == 0 {
				continue
			}
			if img == nil {
				img = images.NewImage(sz.WidthPx, sz.HeightPx)
			}
			t.Draw(img.RGBA)
			img.Upload()
			glctx.ClearColor(0, 0, 0, 1)
			glctx.Clear(gl.COLOR_BUFFER_BIT)
			img.Draw(sz, geom.Point{}, geom.Point{X: sz.WidthPt}, geom.Point{Y: sz.HeightPt}, img.RGBA.Bounds())
			a.Publish()

		case key.Event:
			if t.Sess != nil && t.Key(e) {
				repaint()
			}

		case mouse.Event:
			if t.Sess != nil && t.Mouse(e) {
				repaint()
			}

		case exited:
			return nil
		}
	}
	return nil
}
