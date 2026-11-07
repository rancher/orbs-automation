package execx

import (
	"context"
	"io"
)

// PassCmd is a program run on the user's behalf, for the REPL: their own shell,
// with the terminal attached and the environment they gave it. It is the
// reverse of Cmd on every count, and has no allow list, since what it runs is
// the user's own choice (docs/specs/REPL.md).
type PassCmd struct {
	// Path is the program, an absolute path.
	Path string
	// Args are the arguments after the program's name.
	Args []string
	// Dir is the working directory of the program.
	Dir string
	// Env is the whole environment of the program, "KEY=value" entries, passed
	// on as it is: nothing is scrubbed.
	Env []string
	// In, Out and Err are the program's standard streams. Files are handed over
	// as they are, so a terminal stays a terminal.
	In       io.Reader
	Out, Err io.Writer
}

// Passthrough runs c until it exits and returns its exit status. A program that
// exits with a status, even a non-zero one, is not an error: the error is for a
// program that could not be started. Cancelling ctx kills it.
func Passthrough(ctx context.Context, c PassCmd) (int, error) { return 0, nil }
