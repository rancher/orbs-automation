package ship

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ErrNothingChanged is returned by Apply.Run when the edit and the steps left
// nothing to commit, so the branch was already up to date.
var ErrNothingChanged = errors.New("the edit changed nothing")

// Apply is the edit and commit of one planned target, in a prepared work
// directory.
type Apply struct {
	Invocation updatedispatcher.Invocation // the route's method bound to the edge's params
	Release    updatedispatcher.Release    // what the value is made from
	Branch     string                      // the consumer branch, for the steps' line ranges
	Message    string                      // the commit message, as the preview showed it
	Runner     updatedispatcher.StepRunner // runs the steps
	Name       string                      // the commit's author and committer, or empty for the clone's git config
	Email      string
	Sign       bool
}

// ApplyResult is what a run did.
type ApplyResult struct {
	Commit  string                    // the new commit
	Changes []updatedispatcher.Change // what the strategy changed
}

// Run runs the dispatcher's lifecycle in w.Dir (the pre-edit steps, the edit and
// the post-edit steps) and commits what changed. A step that fails is the
// dispatcher's *StepError, and the caller aborts the work. A run that changes
// nothing is ErrNothingChanged.
func (a Apply) Run(ctx context.Context, w *Work) (ApplyResult, error) {
	res, err := a.Invocation.Run(ctx, updatedispatcher.DirTree(w.Dir), a.Runner, a.Release, a.Branch, w.Dir)
	if err != nil {
		return ApplyResult{}, err
	}
	commit, err := w.Commit(ctx, GitCommit{Message: a.Message, Name: a.Name, Email: a.Email, Sign: a.Sign})
	if errors.Is(err, gitcli.ErrNothingToCommit) {
		return ApplyResult{}, ErrNothingChanged
	}
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Commit: commit, Changes: res.Changes}, nil
}
