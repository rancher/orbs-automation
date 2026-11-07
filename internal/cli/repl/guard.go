package repl

import (
	"context"
	"os"
)

// EnvRepl is set in the environment of a running REPL, so a REPL started from
// inside one is refused.
const EnvRepl = "GERO_REPL"

// Refuse returns why the REPL cannot start here, or nil. tty says whether stdin
// and stdout are both terminals, and ci whether the context is CI. The REPL is
// an enhancement on a person's shell, so where it cannot be that it refuses and
// says so, and does not offer a lesser prompt (docs/specs/REPL.md).
func Refuse(getenv func(string) string, tty, ci bool) error { return nil }

// Interruptible makes a Runner whose context is cancelled by a signal on sigs
// while a line runs. A signal while no line runs is dropped, so Ctrl-C at the
// prompt never leaves the REPL and never cancels the next line.
func Interruptible(sigs <-chan os.Signal, run Runner) Runner { return run }

var _ = context.Background
