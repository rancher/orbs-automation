package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/platform/workdir"
	"github.com/rancher/orbs-automation/internal/task/bumpcommit"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ErrNothingChanged is returned by Apply.Run when the edit and the steps left
// nothing to commit, so the branch was already up to date.
var ErrNothingChanged = bumpcommit.ErrNothingChanged

// Apply is the edit and commit of one planned target, in a prepared work
// directory.
type Apply struct {
	Invocation updatedispatcher.Invocation // the route's method bound to the edge's params
	Release    updatedispatcher.Release    // what the value is made from
	Branch     string                      // the consumer branch, for the steps' line ranges
	Message    string                      // the commit message, as the preview showed it
	// OnCommit, when set, is called with each commit as it is made.
	OnCommit func(CommitInfo)
	// StepMessage is the commit message for a step that changed files.
	StepMessage func(updatedispatcher.Unit) (string, error)
	Runner      updatedispatcher.StepRunner // runs the steps
	Name        string                      // the commit's author and committer, or empty for the clone's git config
	Email       string
	Sign        bool
}

// CommitInfo is a commit a run made.
type CommitInfo = workdir.CommitInfo

// ApplyResult is what a run did.
type ApplyResult = bumpcommit.Output

// tip is the newest commit, or "" when there is none.
func tip(commits []CommitInfo) string {
	if len(commits) == 0 {
		return ""
	}
	return commits[len(commits)-1].Hash
}

// Run applies the edit and commits it with the bumpcommit task, in w.Dir. A step
// that fails is the dispatcher's *StepError, and the caller aborts the work,
// which removes the commits made so far. A run in which nothing changed is
// ErrNothingChanged.
func (a Apply) Run(ctx context.Context, w *Work) (ApplyResult, error) {
	return bumpcommit.Run(ctx, a.Runner, bumpcommit.Input{
		Work: w, Invocation: a.Invocation, Release: a.Release, Branch: a.Branch, Message: a.Message,
		StepMessage: a.StepMessage, OnCommit: a.OnCommit, Name: a.Name, Email: a.Email, Sign: a.Sign,
	})
}
