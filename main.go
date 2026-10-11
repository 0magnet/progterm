//go:build !js

// Command progterm is a desktop progressive terminal: your shell on a real
// pty, drawn by websh's host in a browser window. See the commands package.
package main

import (
	"embed"

	"github.com/0magnet/progterm/commands"
)

//go:embed page
var page embed.FS

func main() { commands.Execute(page) }
