package backport

import (
	"context"
	"io"
	"log/slog"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// Options is what the user asked to backport; the command fills it from its arguments and flags.
type Options struct {
	Args    Args
	Preview bool // print the PR bodies too
	NoPush  bool
	PushTo  string
	Yes     bool
	// Interactive says In is a terminal that questions can be asked on, a conflict included.
	Interactive bool
	In          io.Reader
}

// Run carries a merged PR to the release lines named: it checks the PR on the host, makes the
// picks for each target and, unless --no-push or --dry-run, pushes each branch and opens its PR
// after asking once. A conflict waits for the person at the terminal. The report is written to w,
// as text or as one document in json, and it is written even when the error returned is ErrFailed
// or ErrDeclined.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts Options) error {
	return nil
}

// runTo backports, with an executor, and writes the report. Errors that come after the check (no
// clone, a failed target, declined) are returned after it.
func runTo(ctx context.Context, w io.Writer, ws domain.Workspace, ex *proposal.Executor, format string, in domain.Input, full bool, st *style.Renderer) error {
	return nil
}
