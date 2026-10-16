//go:build unix && !js && !android && !ios

package commands

import (
	"github.com/spf13/cobra"

	"github.com/0magnet/progterm/glterm"
	"github.com/0magnet/progterm/x11term"
)

var (
	x11Shell     string
	x11Font      string
	x11FontPt    float64
	x11InProcess bool
)

var x11Cmd = &cobra.Command{
	Use:   "x11 [-- ARG...]",
	Short: "the terminal in an X11 window, with no cgo or X libraries",
	Long: "x11 runs the shell in a window on $DISPLAY: the same terminal as gio, with the\n" +
		"X protocol spoken in Go, so it needs no cgo and no X libraries. It works on\n" +
		"an X server or XWayland, local or remote. Arguments after -- go to the shell.",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		sh := x11Shell
		if sh == "" {
			sh = glterm.DefaultShell()
		}
		opt := glterm.Options{Shell: sh, Args: args, Font: x11Font, FontPt: x11FontPt}
		if x11InProcess {
			opt.Run = websh
		}
		return x11term.Run(opt)
	},
}

func init() {
	f := x11Cmd.Flags()
	f.StringVar(&x11Shell, "shell", "", "the shell to run (default: websh if installed, else $SHELL)")
	f.StringVar(&x11Font, "font", "", "the font, \"Family Size\" (default: the desktop terminal's, else Go Mono)")
	f.Float64Var(&x11FontPt, "font-size", 0, "font size in points, in place of the font's")
	f.BoolVar(&x11InProcess, "in-process", false, "run websh in this process in each tab, on a pty of its own, in place of a shell program")
	RootCmd.AddCommand(x11Cmd)
}
