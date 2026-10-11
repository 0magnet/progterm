//go:build !js

// Package commands is the progterm command, a desktop progressive terminal: your shell, on a real pty on
// this machine, drawn by websh's host in a browser window — so native
// programs (vim, htop, ssh, anything) run as in any terminal, and a
// progressive one (PROTOCOL.md in websh) gets pictures, widgets it ships,
// events and the rest, as it does in websh.
//
// It serves the host page on a loopback address, opens it as an app window,
// and joins the page to a shell on a pty over a WebSocket. Each window is
// its own shell. Only the page it opened can reach that shell: the address
// is loopback, a fresh token is in the window's link (its fragment, which no
// request carries), and a request naming another host or origin is refused.
package commands

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
	"github.com/spf13/cobra"

	"github.com/0magnet/calvin/clihelp"
)

// page is the host page, embedded by the main package (an embed cannot reach
// outside its own directory) and handed over by Execute.
var page embed.FS

var (
	addr    string
	sh      string
	noOpen  bool
	browser string
)

// RootCmd is the progterm command.
var RootCmd = &cobra.Command{
	Use:           "progterm",
	Short:         "a desktop progressive terminal: your shell in an app window",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if sh == "" {
			sh = os.Getenv("SHELL")
		}
		if sh == "" {
			sh = "/bin/sh"
		}
		return run(addr, sh, noOpen, browser)
	},
}

func init() {
	f := RootCmd.Flags()
	f.StringVar(&addr, "addr", "127.0.0.1:0", "loopback address to serve the window on")
	f.StringVar(&sh, "shell", "", "the shell to run (default: $SHELL, else /bin/sh)")
	f.BoolVar(&noOpen, "no-open", false, "print the window's address instead of opening it")
	f.StringVar(&browser, "browser", "", "the browser to open the window with (default: the first Chromium-family one found, else xdg-open)")
	clihelp.Init(RootCmd, "progterm")
}

// Execute runs the command, serving the given host page, and exits non-zero
// on error.
func Execute(host embed.FS) {
	page = host
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "progterm:", err)
		os.Exit(1)
	}
}

func run(addr, sh string, noOpen bool, browser string) error {
	if _, err := page.Open("page/host.wasm"); err != nil {
		return errors.New("the host page is not built in: run make, which builds page/host.wasm first")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return errors.New("--addr must be a loopback address: the window is a shell on this machine")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	b := &bridge{
		hostport: ln.Addr().String(),
		token:    newToken(),
		shell:    sh,
		idle:     make(chan struct{}, 1),
	}
	srv := &http.Server{Handler: b.handler(), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln) //nolint:errcheck // ends with the process

	url := "http://" + b.hostport + "/#" + b.token
	if noOpen {
		fmt.Println(url)
	} else if err := open(url, browser); err != nil {
		fmt.Println("progterm: could not open a window (" + err.Error() + "); open " + url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-b.idle: // the last window's shell is done
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// bridge joins host pages to shells.
type bridge struct {
	hostport string // 127.0.0.1:port, the only Host a request may name
	token    string
	shell    string

	mu       sync.Mutex
	sessions int
	opened   bool
	idle     chan struct{}
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // no randomness: nothing safe to do
	}
	return hex.EncodeToString(b)
}

func (b *bridge) handler() http.Handler {
	files, _ := fs.Sub(page, "page") //nolint:errcheck // the directory is embedded
	static := http.FileServer(http.FS(files))
	mux := http.NewServeMux()
	mux.HandleFunc("/pty", b.pty)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		static.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A page elsewhere that points a name of its own at 127.0.0.1 (DNS
		// rebinding) names that host, not this one.
		if r.Host != b.hostport {
			http.Error(w, "wrong host", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// pty is one window's shell: binary messages are the pty's bytes both ways,
// a text message {"cols","rows"} its size.
func (b *bridge) pty(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(b.token)) != 1 {
		http.Error(w, "no", http.StatusForbidden)
		return
	}
	if r.Header.Get("Origin") != "http://"+b.hostport {
		http.Error(w, "wrong origin", http.StatusForbidden)
		return
	}
	c, err := websocket.Accept(w, r, nil) // nil options: the origin must be this host's
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20)
	b.started()
	defer b.ended()

	cmd := exec.Command(b.shell, "-l") //nolint:gosec // the user's own shell, by their flag or $SHELL
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=progterm")
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		c.Close(websocket.StatusInternalError, "could not start the shell") //nolint:errcheck,gosec // closing
		log.Println("progterm: starting", b.shell+":", err)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// The shell's output, to the window, until it exits.
	go func() {
		defer cancel()
		buf := make([]byte, 32<<10)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				if c.Write(ctx, websocket.MessageBinary, buf[:n]) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	// The window's keys and size, to the shell, until the window goes.
	go func() {
		defer cancel()
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageText {
				var sz struct{ Cols, Rows uint16 }
				if json.Unmarshal(data, &sz) == nil && sz.Cols > 0 && sz.Rows > 0 {
					pty.Setsize(f, &pty.Winsize{Cols: sz.Cols, Rows: sz.Rows}) //nolint:errcheck,gosec // best effort, as a terminal's resize
				}
				continue
			}
			if _, err := f.Write(data); err != nil {
				return
			}
		}
	}()
	<-ctx.Done()
	// A window closed is a terminal hung up on.
	cmd.Process.Signal(syscall.SIGHUP)                         //nolint:errcheck,gosec // it may have exited already
	f.Close()                                                  //nolint:errcheck,gosec // done with it
	cmd.Wait()                                                 //nolint:errcheck,gosec // its status is the shell's own
	c.Close(websocket.StatusNormalClosure, "the shell exited") //nolint:errcheck,gosec // closing
}

func (b *bridge) started() {
	b.mu.Lock()
	b.sessions++
	b.opened = true
	b.mu.Unlock()
}

// ended counts a window's shell done; with none left, a moment later, the
// bridge is done too (a reload reconnects within it).
func (b *bridge) ended() {
	b.mu.Lock()
	b.sessions--
	b.mu.Unlock()
	time.AfterFunc(3*time.Second, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.opened && b.sessions == 0 {
			select {
			case b.idle <- struct{}{}:
			default:
			}
		}
	})
}

// open shows url as an app window: a Chromium-family browser's --app, which
// has no address bar or tabs, else the desktop's default browser.
func open(url, browser string) error {
	if browser != "" {
		return exec.Command(browser, "--app="+url).Start() //nolint:gosec // the user's own flag
	}
	for _, name := range []string{"brave", "brave-browser", "chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			return exec.Command(p, "--app="+url).Start() //nolint:gosec // a browser found on PATH
		}
	}
	return exec.Command("xdg-open", url).Start() //nolint:gosec // our own loopback address
}
