// Package bumpcommit is the task that applies one change set to a prepared
// branch and commits it: the edit and its commits are never separated. One
// action, run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package bumpcommit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/workdir"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ErrNothingChanged is returned when the edit and the steps left nothing to
// commit, so the branch was already up to date.
var ErrNothingChanged = errors.New("the edit changed nothing")

// Input is the change to apply, and where.
type Input struct {
	// Work is the branch to edit and commit in, as createbranch made it.
	Work       *workdir.Work
	Invocation updatedispatcher.Invocation // the route's method bound to the edge's params
	Release    updatedispatcher.Release    // what the value is made from
	Branch     string                      // the consumer branch, for the steps' line ranges
	Message    string                      // the commit message of the edit
	// StepMessage is the commit message for a step that changed files. Without it
	// the message is just what ran.
	StepMessage func(updatedispatcher.Unit) (string, error)
	// OnCommit, when set, is called with each commit as it is made.
	OnCommit func(workdir.CommitInfo)
	Name     string // the commit's author and committer, or empty for the clone's git config
	Email    string
	Sign     bool
}

// Output is what the run did.
type Output struct {
	Commits []workdir.CommitInfo      // the commits made, oldest first
	Changes []updatedispatcher.Change // what the strategy changed
}

// Tip is the newest commit, or "" when there is none.
func (o Output) Tip() string {
	if len(o.Commits) == 0 {
		return ""
	}
	return o.Commits[len(o.Commits)-1].Hash
}

// Run runs the dispatcher's lifecycle in the work directory (the pre-edit steps,
// the edit and the post-edit steps) and commits after each of them that changed
// files: the edit with Message, a step with StepMessage. A unit that changes
// nothing makes no commit. A step that fails is the dispatcher's *StepError, and
// the caller aborts the work, which removes the commits made so far. A run in
// which nothing changed is ErrNothingChanged.
func Run(ctx context.Context, runner updatedispatcher.StepRunner, in Input) (Output, error) {
	w := in.Work
	var commits []workdir.CommitInfo
	after := func(ctx context.Context, u updatedispatcher.Unit) error {
		message := in.Message
		if u.Kind == updatedispatcher.UnitStep {
			var err error
			if message, err = messageFor(in, u); err != nil {
				return fmt.Errorf("the commit message for %s: %w", strings.Join(u.Run, " "), err)
			}
		}
		hash, err := w.Commit(ctx, workdir.GitCommit{Message: message, Name: in.Name, Email: in.Email, Sign: in.Sign})
		if errors.Is(err, gitcli.ErrNothingToCommit) {
			return nil
		}
		if err != nil {
			return err
		}
		subject, _, _ := strings.Cut(message, "\n")
		commit := workdir.CommitInfo{Hash: hash, Subject: subject}
		// The files are for the report. A failure to list them leaves them out and
		// never fails a commit that was made.
		if files, err := w.CommitFiles(ctx, hash); err == nil {
			commit.Files = files
		}
		commits = append(commits, commit)
		if in.OnCommit != nil {
			in.OnCommit(commit)
		}
		return nil
	}
	res, err := in.Invocation.Run(ctx, updatedispatcher.DirTree(w.Dir), runner, in.Release, in.Branch, w.Dir, after)
	if err != nil {
		return Output{}, err
	}
	if len(commits) == 0 {
		return Output{}, ErrNothingChanged
	}
	return Output{Commits: commits, Changes: res.Changes}, nil
}

// messageFor is the commit message of a step, from StepMessage or, when there
// is none, just what ran.
func messageFor(in Input, u updatedispatcher.Unit) (string, error) {
	if in.StepMessage == nil {
		return "Run `" + strings.Join(u.Run, " ") + "`", nil
	}
	return in.StepMessage(u)
}
