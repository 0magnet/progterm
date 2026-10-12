//go:build cgo && unix

package commands

import (
	"os"

	"github.com/0magnet/websh/console"

	"github.com/0magnet/progterm/gioterm"
	"github.com/0magnet/progterm/glterm"
)

func runGio(opt glterm.Options, inProcess bool) error {
	if inProcess {
		opt.Run = websh
	}
	return gioterm.Run(opt)
}

// websh is websh's console on a tab's pty, in this process.
func websh(tty *os.File, sig <-chan os.Signal) int {
	code, _ := console.Run(console.Options{ //nolint:errcheck // a tab shows its own errors
		In: tty, Out: tty, Signals: sig,
		Env: []string{"TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=progterm"},
	})
	return code
}
