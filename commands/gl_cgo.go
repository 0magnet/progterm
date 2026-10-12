//go:build cgo && unix

package commands

import "github.com/0magnet/progterm/glterm"

func runGL(opt glterm.Options) error { return glterm.Run(opt) }
