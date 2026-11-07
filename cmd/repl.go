package cmd

import (
	"context"
	"io"

	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/spf13/cobra"
)

// addRepl adds gero repl to root.
func addRepl(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "repl",
		Short: "Run gero commands one line at a time at a prompt",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return nil },
	})
}

// replEnv is what runRepl takes from the process, so a test can give it others.
type replEnv struct {
	tty    bool
	getenv func(string) string
	setenv func(key, value string) error
}

// lineRunner returns the Runner of the REPL: each line is run on a command tree
// built for it, with base, the global flags given to gero repl, in front of the
// words of the line. The tree reads in and writes to out and errOut.
func lineRunner(base []string, in io.Reader, out, errOut io.Writer) repl.Runner {
	return func(context.Context, string, []string) error { return nil }
}
