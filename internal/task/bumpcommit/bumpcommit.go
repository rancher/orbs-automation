// Package bumpcommit is the task that applies one change set to a prepared
// branch and commits it: the edit and its commits are never separated. One
// action, run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package bumpcommit

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workdir"
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

// Run applies the change. TODO: the stub does nothing yet.
func Run(_ context.Context, _ updatedispatcher.StepRunner, _ Input) (Output, error) {
	return Output{}, nil
}
