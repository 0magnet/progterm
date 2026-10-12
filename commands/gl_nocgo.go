//go:build !js && !(cgo && unix)

package commands

import (
	"errors"

	"github.com/0magnet/progterm/glterm"
)

func runGL(glterm.Options) error {
	return errors.New("gl needs a build with cgo (CGO_ENABLED=1), X11 and OpenGL ES")
}
