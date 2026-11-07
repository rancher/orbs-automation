package repl

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

// EnvRepl is set in the environment of a running REPL, so a REPL started from
// inside one is refused.
const EnvRepl = "GERO_REPL"

// Refuse returns why the REPL cannot start here, or nil. term is what the
// terminal can do, and ci whether the context is CI. The REPL is
// an enhancement on a person's shell, so where it cannot be that it refuses and
// says so, and does not offer a lesser prompt (docs/specs/REPL.md).
func Refuse(getenv func(string) string, term clicfg.Terminal, ci bool) error {
	switch {
	case !term.StdinTTY || !term.StdoutTTY:
		return errors.New("gero repl needs a terminal: stdin and stdout must both be one. Use your shell instead")
	case ci:
		return errors.New("gero repl does not run in CI")
	case getenv(EnvRepl) != "":
		return errors.New("already in a gero repl (" + EnvRepl + " is set): exit it first, or use your shell")
	}
	if !term.Cursor {
		return errors.New("gero repl needs a terminal that can move the cursor, and TERM is unset or dumb. " +
			"Use your shell instead")
	}
	return nil
}

// Interruptible makes a Runner whose context is cancelled by a signal on sigs
// while a line runs. A signal while no line runs is dropped, so Ctrl-C at the
// prompt never leaves the REPL and never cancels the next line.
func Interruptible(sigs <-chan os.Signal, run Runner) Runner {
	var mu sync.Mutex
	var cancel context.CancelFunc
	go func() {
		for range sigs {
			mu.Lock()
			if cancel != nil {
				cancel()
			}
			mu.Unlock()
		}
	}()
	return func(ctx context.Context, cwd string, args []string) error {
		ctx, c := context.WithCancel(ctx)
		mu.Lock()
		cancel = c
		mu.Unlock()
		defer func() {
			mu.Lock()
			cancel = nil
			mu.Unlock()
			c()
		}()
		return run(ctx, cwd, args)
	}
}
