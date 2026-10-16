//go:build unix && !js && !cgo

package commands

import (
	"errors"

	"github.com/0magnet/progterm/glterm"
)

func runGio(glterm.Options, bool) error {
	return errors.New("gio needs a build with cgo (CGO_ENABLED=1)")
}
