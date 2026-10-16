//go:build !js

package commands

import (
	"github.com/spf13/cobra"

	"github.com/0magnet/progterm/glterm"
)

var (
	gioShell     string
	gioFont      string
	gioFontPt    float64
	gioInProcess bool
)

var gioCmd = &cobra.Command{
	Use:   "gio [-- ARG...]",
	Short: "the terminal in a Gio window (the gl terminal on Gio instead of gomobile)",
	Long: "gio runs the shell in a window Gio draws: the same terminal as gl, with Gio\n" +
		"underneath for the window, input, clipboard and on-screen keyboard.\n" +
		"Arguments after -- go to the shell.",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		sh := gioShell
		if sh == "" {
			sh = glterm.DefaultShell()
		}
		return runGio(glterm.Options{Shell: sh, Args: args, Font: gioFont, FontPt: gioFontPt}, gioInProcess)
	},
}

func init() {
	f := gioCmd.Flags()
	f.StringVar(&gioShell, "shell", "", "the shell to run (default: websh if installed, else $SHELL)")
	f.StringVar(&gioFont, "font", "", "the font, \"Family Size\" (default: the desktop terminal's, else Go Mono)")
	f.Float64Var(&gioFontPt, "font-size", 0, "font size in points, in place of the font's")
	f.BoolVar(&gioInProcess, "in-process", false, "run websh in this process in each tab, on a pty of its own, in place of a shell program")
	RootCmd.AddCommand(gioCmd)
}
