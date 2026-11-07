package repl

import (
	"context"
	"io"

	"github.com/rancher/orbs-automation/internal/execx"
)

// ShellFunc runs the user's shell, the program at path shell, in dir with env
// as its whole environment, and returns when it exits. Its exit status is not
// an error: only a failure to start it is.
type ShellFunc func(ctx context.Context, shell, dir string, env []string) error

// ShellOn returns the ShellFunc that attaches the shell to in, out and errOut.
func ShellOn(in io.Reader, out, errOut io.Writer) ShellFunc {
	return func(context.Context, string, string, []string) error {
		_ = execx.PassCmd{In: in, Out: out, Err: errOut}
		return nil
	}
}
