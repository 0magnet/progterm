//go:build cgo && unix

package glterm

import (
	"fmt"
	"sync/atomic"

	"github.com/0magnet/mobile/app"
	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/mobile/event/lifecycle"
	"github.com/0magnet/mobile/event/paint"
	"github.com/0magnet/mobile/event/size"
	"github.com/0magnet/mobile/exp/gl/glutil"
	"github.com/0magnet/mobile/geom"
	"github.com/0magnet/mobile/gl"
)

// exited is sent through the app's events when the shell has ended.
type exited struct{}

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
		grid   *Grid
		ppp    float32
		sess   *Session
		dirty  atomic.Bool // a paint is wanted and not yet sent
	)
	defer func() {
		if sess != nil {
			sess.Close() //nolint:errcheck,gosec // the window is going
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
			if grid == nil || e.PixelsPerPt != ppp {
				g, err := NewGrid(opt.FontPt * float64(e.PixelsPerPt))
				if err != nil {
					return err
				}
				grid, ppp = g, e.PixelsPerPt
			}
			cols, rows := grid.Size(e.WidthPx, e.HeightPx)
			if sess == nil {
				s, err := Start(opt.Shell, opt.Args, cols, rows)
				if err != nil {
					return fmt.Errorf("glterm: %w", err)
				}
				sess = s
				sess.OnChange = repaint
				go func() {
					<-s.Done()
					a.Send(exited{})
				}()
			} else {
				sess.Resize(cols, rows, e.WidthPx, e.HeightPx)
			}
			if img != nil {
				img.Release()
				img = nil
			}
			repaint()

		case paint.Event:
			dirty.Store(false)
			if glctx == nil || sess == nil || sz.WidthPx == 0 {
				continue
			}
			if img == nil {
				img = images.NewImage(sz.WidthPx, sz.HeightPx)
			}
			sess.Lock()
			grid.Draw(sess.Term, img.RGBA)
			sess.Unlock()
			img.Upload()
			glctx.ClearColor(0, 0, 0, 1)
			glctx.Clear(gl.COLOR_BUFFER_BIT)
			img.Draw(sz, geom.Point{}, geom.Point{X: sz.WidthPt}, geom.Point{Y: sz.HeightPt}, img.RGBA.Bounds())
			a.Publish()

		case key.Event:
			if sess == nil {
				continue
			}
			sess.Lock()
			appCursor := sess.Term.CoreService().DecPrivateModes.ApplicationCursorKeys
			sess.Unlock()
			sess.Input(Encode(e, appCursor))

		case exited:
			return nil
		}
	}
	return nil
}
