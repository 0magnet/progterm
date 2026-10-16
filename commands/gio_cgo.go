//go:build cgo && unix

package commands

import (
	"github.com/0magnet/progterm/gioterm"
	"github.com/0magnet/progterm/glterm"
)

func runGio(opt glterm.Options, inProcess bool) error {
	if inProcess {
		opt.Run = websh
	}
	return gioterm.Run(opt)
}
