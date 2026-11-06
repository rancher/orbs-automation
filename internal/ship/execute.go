package ship

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ResultStatus is what a run did for one target.
type ResultStatus string

// The results.
const (
	// ResultCommitted: the commit is on the new branch in the clone.
	ResultCommitted ResultStatus = "committed"
	// ResultWouldCommit: a dry run made the commit in a worktree and removed it.
	ResultWouldCommit ResultStatus = "would commit"
	// ResultUpToDate: the edit and the steps changed nothing.
	ResultUpToDate ResultStatus = "already up to date"
	// ResultFailed: preparing the branch, a step or the commit failed.
	ResultFailed ResultStatus = "failed"
)

// TargetResult is what a run did for one target that needed a PR.
type TargetResult struct {
	Status  ResultStatus `json:"status"`
	Branch  string       `json:"branch"`            // the new branch
	Commit  string       `json:"commit,omitempty"`  // the commit made
	Place   Place        `json:"place,omitempty"`   // where it was made
	Reason  string       `json:"reason,omitempty"`  // why that was a worktree
	Changes []string     `json:"changes,omitempty"` // what the strategy changed, in words
	Error   string       `json:"error,omitempty"`   // why it failed
}

// Executor is what a run needs to make commits.
type Executor struct {
	Git     Git
	Runner  updatedispatcher.StepRunner
	TempDir string // where worktrees go
}

// ErrNoClone is returned by Ship when there are commits to make and the
// engineer has no clone of the consumer to make them in.
var ErrNoClone = errors.New("no local clone of the consumer")

// Ship checks the state, as Check does, and then, when ex is not nil and
// nothing blocks the release as a whole, makes the commit for each target that
// needs one: in the engineer's clone or a worktree of it. A dry run makes the
// commit in a worktree and removes it. Nothing is pushed.
//
// STUB: only checks until a later commit.
func Ship(ctx context.Context, ws Workspace, prober registry.Prober, in Input, ex *Executor) (Plan, error) {
	plan, _, err := check(ctx, ws, prober, in)
	return plan, err
}
