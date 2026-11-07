// Package termfacts asks the process about its terminal, the facts that
// clicfg.Load cannot read itself because it does no I/O: whether stdin and
// stdout are terminals, and how wide stdout is. It is the one place that
// imports golang.org/x/term (docs/specs/DEPENDENCY_POLICY.md).
package termfacts

import (
	"os"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"golang.org/x/term"
)

// IsTerminal reports whether f is a terminal, asked of the terminal driver. A
// pipe, a file and /dev/null are not.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// Detect returns the facts of the process for stdin and stdout. The width is
// that of stdout when it is a terminal, and 0 otherwise.
func Detect(stdin, stdout *os.File) clicfg.Facts {
	f := clicfg.Facts{StdinTTY: IsTerminal(stdin), StdoutTTY: IsTerminal(stdout)}
	if f.StdoutTTY {
		if w, _, err := term.GetSize(int(stdout.Fd())); err == nil {
			f.Width = w
		}
	}
	return f
}
