package ship

import (
	"context"
	"errors"
	"strings"

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
	// StepMessage is the commit message for a step that changed files.
	//
	// STUB: not used until a later commit.
	StepMessage func(updatedispatcher.Unit) (string, error)
	Runner      updatedispatcher.StepRunner // runs the steps
	Name        string                      // the commit's author and committer, or empty for the clone's git config
	Email       string
	Sign        bool
}

// CommitInfo is a commit a run made.
type CommitInfo struct {
	Hash    string `json:"commit"`
	Subject string `json:"subject"`
}

// ApplyResult is what a run did.
type ApplyResult struct {
	Commits []CommitInfo              // the commits made, oldest first
	Changes []updatedispatcher.Change // what the strategy changed
}

// Tip is the newest commit, or "" when there is none.
func (r ApplyResult) Tip() string { return tip(r.Commits) }

func tip(commits []CommitInfo) string {
	if len(commits) == 0 {
		return ""
	}
	return commits[len(commits)-1].Hash
}

// Run runs the dispatcher's lifecycle in w.Dir (the pre-edit steps, the edit and
// the post-edit steps) and commits what changed. A step that fails is the
// dispatcher's *StepError, and the caller aborts the work. A run that changes
// nothing is ErrNothingChanged.
func (a Apply) Run(ctx context.Context, w *Work) (ApplyResult, error) {
	res, err := a.Invocation.Run(ctx, updatedispatcher.DirTree(w.Dir), a.Runner, a.Release, a.Branch, w.Dir, nil)
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
	subject, _, _ := strings.Cut(a.Message, "\n")
	return ApplyResult{Commits: []CommitInfo{{Hash: commit, Subject: subject}}, Changes: res.Changes}, nil
}
