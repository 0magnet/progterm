//go:build js && wasm

// The host page: websh's progressive terminal host, whose one command joins
// the bridge's pty. It runs as the ssh applet does, raw, the bytes relayed
// both ways and the size followed, its output marked a terminal's (the
// remote rules, and the window's title), since the host cannot see whether
// they are this machine's or an ssh session's inside the shell.
package main

import (
	"context"
	"encoding/json"
	"sync"
	"syscall/js"
	"time"

	"github.com/0magnet/sh/v3/interp"
	"github.com/0magnet/websh/web"
)

func main() {
	doc := js.Global().Get("document")
	// The token is in the fragment, which no request carries; it leaves the
	// address once read.
	token := js.Global().Get("location").Get("hash").String()
	if len(token) > 0 && token[0] == '#' {
		token = token[1:]
	}
	js.Global().Get("history").Call("replaceState", js.Null(), "", "/")
	s, err := web.NewSession(doc.Call("getElementById", "terminal"), web.Options{
		Host:     "progterm",
		NoKeyBar: true,
		Exec: func(ctx context.Context, args []string) (int, bool) {
			if len(args) == 0 || args[0] != "progterm-pty" {
				return 0, false
			}
			return join(ctx, token), true
		},
	})
	if err != nil {
		doc.Get("body").Set("textContent", "progterm: "+err.Error())
		return
	}
	s.Submit("progterm-pty")
	select {}
}

// join relays this terminal to the bridge's pty until the shell exits or
// the connection ends, and then closes the window.
func join(ctx context.Context, token string) int {
	sess := web.SessionForContext(ctx)
	hc := interp.HandlerCtx(ctx)
	sh := sess.Shell
	defer sh.WithSource("terminal")()
	if sh.RawMode != nil {
		sh.RawMode(true)
		defer sh.RawMode(false)
	}
	hc.Stdout.Write([]byte("\x1b[H\x1b[2J")) //nolint:errcheck,gosec // the page's own prompt goes; the shell's comes

	loc := js.Global().Get("location")
	ws := js.Global().Get("WebSocket").New("ws://" + loc.Get("host").String() + "/pty?t=" + token)
	ws.Set("binaryType", "arraybuffer")
	in := newQueue()
	open := make(chan struct{})
	var once sync.Once
	onOpen := js.FuncOf(func(js.Value, []js.Value) any { close(open); return nil })
	onMsg := js.FuncOf(func(_ js.Value, args []js.Value) any {
		u := js.Global().Get("Uint8Array").New(args[0].Get("data"))
		b := make([]byte, u.Length())
		js.CopyBytesToGo(b, u)
		in.push(b)
		return nil
	})
	onEnd := js.FuncOf(func(js.Value, []js.Value) any {
		once.Do(func() {
			in.close()
			if sh.WakeStdin != nil {
				sh.WakeStdin() // the keys' read ends too
			}
		})
		return nil
	})
	defer onOpen.Release()
	defer onMsg.Release()
	defer onEnd.Release()
	ws.Set("onopen", onOpen)
	ws.Set("onmessage", onMsg)
	ws.Set("onclose", onEnd)
	ws.Set("onerror", onEnd)

	select {
	case <-open:
	case <-in.done:
		hc.Stdout.Write([]byte("progterm: the shell could not be reached\r\n")) //nolint:errcheck,gosec // the terminal
		return 1
	}
	send := func(b []byte) {
		u := js.Global().Get("Uint8Array").New(len(b))
		js.CopyBytesToJS(u, b)
		ws.Call("send", u)
	}
	resize := func(cols, rows int) {
		b, _ := json.Marshal(map[string]int{"cols": cols, "rows": rows}) //nolint:errcheck // two ints
		ws.Call("send", string(b))
	}

	// The size, now and as it changes.
	cols, rows := 80, 24
	if sh.Size != nil {
		cols, rows = sh.Size()
	}
	resize(cols, rows)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		tk := time.NewTicker(200 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
			}
			if sh.Size == nil {
				continue
			}
			if c, r := sh.Size(); c != cols || r != rows {
				cols, rows = c, r
				resize(c, r)
			}
		}
	}()
	// The keys, to the shell.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := hc.Stdin.Read(buf)
			if n > 0 {
				send(append([]byte(nil), buf[:n]...))
			}
			if err != nil || in.closed() {
				return
			}
		}
	}()
	// The shell's output, to the terminal, until it ends.
	for {
		b, ok := in.pop()
		if !ok {
			break
		}
		hc.Stdout.Write(b) //nolint:errcheck,gosec // the terminal
	}
	ws.Call("close")
	js.Global().Call("close")                                                // the window was the shell's; an app window closes
	hc.Stdout.Write([]byte("\r\n[the shell exited: close this window]\r\n")) //nolint:errcheck,gosec // a window a script may not close stays
	return 0
}

// queue holds what the pty sent until the terminal takes it: a JS callback
// must not block.
type queue struct {
	mu    sync.Mutex
	items [][]byte
	more  chan struct{}
	done  chan struct{}
	ended bool
}

func newQueue() *queue { return &queue{more: make(chan struct{}, 1), done: make(chan struct{})} }

func (q *queue) push(b []byte) {
	q.mu.Lock()
	q.items = append(q.items, b)
	q.mu.Unlock()
	select {
	case q.more <- struct{}{}:
	default:
	}
}

func (q *queue) close() {
	q.mu.Lock()
	if !q.ended {
		q.ended = true
		close(q.done)
	}
	q.mu.Unlock()
}

func (q *queue) closed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ended
}

// pop is the next output, or false once the connection has ended and all
// it sent is taken.
func (q *queue) pop() ([]byte, bool) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			b := q.items[0]
			q.items = q.items[1:]
			q.mu.Unlock()
			return b, true
		}
		ended := q.ended
		q.mu.Unlock()
		if ended {
			return nil, false
		}
		select {
		case <-q.more:
		case <-q.done:
		}
	}
}
