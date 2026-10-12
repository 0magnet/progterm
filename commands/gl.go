//go:build !js

package commands

import (
	"github.com/spf13/cobra"

	"github.com/0magnet/progterm/glterm"
)

var (
	glShell string
	glFont  float64
)

var glCmd = &cobra.Command{
	Use:   "gl [-- ARG...]",
	Short: "the terminal in a gomobile window, drawn without a browser",
	Long: "gl runs the shell in a window of its own, drawn with OpenGL ES by a gomobile\n" +
		"app: the same app a phone runs. It draws cells only, so a progressive\n" +
		"program draws as it does in any terminal. Arguments after -- go to the shell.",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		sh := glShell
		if sh == "" {
			sh = glterm.DefaultShell()
		}
		return runGL(glterm.Options{Shell: sh, Args: args, FontPt: glFont})
	},
}

func init() {
	f := glCmd.Flags()
	f.StringVar(&glShell, "shell", "", "the shell to run (default: websh if installed, else $SHELL)")
	f.Float64Var(&glFont, "font-size", 13, "font size in points")
	RootCmd.AddCommand(glCmd)
}
