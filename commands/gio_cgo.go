//go:build cgo && unix

package commands

import (
	"github.com/0magnet/progterm/gioterm"
	"github.com/0magnet/progterm/glterm"
)

func runGio(opt glterm.Options) error { return gioterm.Run(opt) }
