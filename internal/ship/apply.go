package ship

import (
	"context"
	"errors"
	"fmt"
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
// the post-edit steps) and commits after each of them that changed files: the
// edit with Message, a step with StepMessage. A unit that changes nothing makes
// no commit. A step that fails is the dispatcher's *StepError, and the caller
// aborts the work, which removes the commits made so far. A run in which nothing
// changed is ErrNothingChanged.
func (a Apply) Run(ctx context.Context, w *Work) (ApplyResult, error) {
	var commits []CommitInfo
	after := func(ctx context.Context, u updatedispatcher.Unit) error {
		message := a.Message
		if u.Kind == updatedispatcher.UnitStep {
			var err error
			if message, err = a.stepMessage(u); err != nil {
				return fmt.Errorf("the commit message for %s: %w", strings.Join(u.Run, " "), err)
			}
		}
		hash, err := w.Commit(ctx, GitCommit{Message: message, Name: a.Name, Email: a.Email, Sign: a.Sign})
		if errors.Is(err, gitcli.ErrNothingToCommit) {
			return nil
		}
		if err != nil {
			return err
		}
		subject, _, _ := strings.Cut(message, "\n")
		commit := CommitInfo{Hash: hash, Subject: subject}
		commits = append(commits, commit)
		if a.OnCommit != nil {
			a.OnCommit(commit)
		}
		return nil
	}
	res, err := a.Invocation.Run(ctx, updatedispatcher.DirTree(w.Dir), a.Runner, a.Release, a.Branch, w.Dir, after)
	if err != nil {
		return ApplyResult{}, err
	}
	if len(commits) == 0 {
		return ApplyResult{}, ErrNothingChanged
	}
	return ApplyResult{Commits: commits, Changes: res.Changes}, nil
}

// stepMessage is the commit message of a step, from StepMessage or, when there
// is none, just what ran.
func (a Apply) stepMessage(u updatedispatcher.Unit) (string, error) {
	if a.StepMessage == nil {
		return "Run `" + strings.Join(u.Run, " ") + "`", nil
	}
	return a.StepMessage(u)
}
