// Package proposer is the wiring that every command that proposes changes shares: it builds
// the executor of a run from the settings and the options, the GitHub side of publishing,
// the credentials a push is made with, the one question asked before anything is written,
// and the workspace's answers about the engineer's clone. It is not a command: gero ship,
// gero ci:ship and gero repo sync each call it, and each keeps its own flags and report
// (docs/specs/PROJECT_LAYOUT.md).
package proposer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/identity"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// Options says how a run publishes, as the user asked.
type Options struct {
	// NoPush stops after the commits, leaving the branches in the clone.
	NoPush bool
	// Yes goes on without asking. Interactive says In is a terminal that the question
	// can be asked on; without it and without Yes a run that writes is an error.
	Yes         bool
	Interactive bool
	In          io.Reader
}

// Build makes the executor of a run: git, the runner of the steps, a directory for
// worktrees, who the commits are made as and, unless the run stops at the commits, the
// publisher and the question. The context it returns carries the credentials a push is
// made with. cleanup removes the directory of worktrees, and is to be called when the run
// is over. A run with no GitHub token is an error, except a dry run, which can still say
// what it would commit but cannot look at the open PRs.
func Build(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, o Options) (context.Context, *proposal.Executor, func(), error) {
	tmp, err := os.MkdirTemp("", "gero-ship-")
	if err != nil {
		return ctx, nil, nil, fmt.Errorf("make a directory for worktrees: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	ex := &proposal.Executor{Git: proposal.GitCLI{}, Runner: steprun.Runner{}, TempDir: tmp, Sign: s.SignCommits, Identify: Identify(s)}
	if o.NoPush {
		return ctx, ex, cleanup, nil
	}
	creds, err := github.Resolve(github.GoGh{}, "")
	switch {
	case err == nil:
		ctx = PushContext(ctx, s, creds)
		remote, err := NewRemote(creds, s)
		if err != nil {
			cleanup()
			return ctx, nil, nil, err
		}
		ex.Publish = &proposal.Publisher{Git: proposal.GitCLI{}, Remote: remote}
		ex.Confirm = Confirmer(o.In, w, o.Interactive, o.Yes, style.New(s.Terminal))
	case s.DryRun && errors.Is(err, github.ErrNoToken):
		// A dry run can still say what it would commit; it just cannot look at
		// the open PRs without a token.
		if log != nil {
			log.Warn("no GitHub token, so the open PRs were not looked at", "error", err)
		}
	default:
		cleanup()
		return ctx, nil, nil, fmt.Errorf("opening PRs needs a GitHub token (--no-push does not): %w", err)
	}
	return ctx, ex, cleanup, nil
}

// Identify says who the commits are made as in the clone: the engineer's git identity
// there on a workstation, the bot's in CI.
func Identify(s clicfg.Settings) func(context.Context, string) (proposal.Identity, error) {
	return func(ctx context.Context, clone string) (proposal.Identity, error) {
		id, err := identity.ResolveIn(ctx, s, clone)
		return proposal.Identity{Name: id.Name, Email: id.Email}, err
	}
}

// ResultRole is the look of what a run did for one branch.
func ResultRole(status proposal.ResultStatus) style.Role {
	switch status {
	case proposal.ResultCommitted, proposal.ResultOpened, proposal.ResultRefreshed:
		return style.OK
	case proposal.ResultFailed:
		return style.Error
	case proposal.ResultBlocked:
		return style.Warn
	}
	return style.Dim
}
