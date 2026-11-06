package ship

import (
	"context"
	"errors"
	"fmt"

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
	Commits []CommitInfo `json:"commits,omitempty"` // the commits made, oldest first
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

// Tip is the newest commit of the result, or "" when there is none.
func (r TargetResult) Tip() string { return tip(r.Commits) }

// ErrNoClone is returned by Ship when there are commits to make and the
// engineer has no clone of the consumer to make them in.
var ErrNoClone = errors.New("no local clone of the consumer")

// Ship checks the state, as Check does, and then, when ex is not nil and
// nothing blocks the release as a whole, makes the commit for each target that
// needs one: in the engineer's clone or a worktree of it. A dry run makes the
// commit in a worktree and removes it. Nothing is pushed.
//
// A target that fails is reported as failed and the others go on. The plan is
// returned with every error that comes after the check, so it can be reported.
func Ship(ctx context.Context, ws Workspace, prober registry.Prober, in Input, ex *Executor) (Plan, error) {
	plan, res, err := check(ctx, ws, prober, in)
	if err != nil || ex == nil || len(plan.Blockers) > 0 {
		return plan, err
	}
	var ready []int
	for i, tp := range plan.Targets {
		if tp.Status == StatusReady && tp.Preview != nil {
			ready = append(ready, i)
		}
	}
	if len(ready) == 0 {
		return plan, nil
	}
	consumer := plan.Consumer
	clone, err := ws.CloneDir(consumer)
	if err != nil {
		return plan, fmt.Errorf("%w: %v", ErrNoClone, err)
	}
	remote, err := ws.Remote(consumer)
	if err != nil {
		return plan, err
	}
	plan.Clone, plan.Remote = clone, remote
	for _, i := range ready {
		plan.Targets[i].Result = res.commitTarget(ctx, ex, clone, remote, in, plan.Targets[i])
	}
	return plan, nil
}

// commitTarget makes the commit for one target and ends the work, whatever
// happened, so the clone is as it was found apart from the branch it keeps.
func (r resolution) commitTarget(ctx context.Context, ex *Executor, clone, remote string, in Input, tp TargetPlan) *TargetResult {
	result := &TargetResult{Branch: tp.Preview.Branch}
	fail := func(err error) *TargetResult {
		result.Status, result.Error = ResultFailed, err.Error()
		return result
	}
	w, err := Prepare(ctx, ex.Git, PrepareRequest{
		CloneDir: clone, Remote: remote, Target: tp.Branch, Base: tp.Commit,
		Branch: tp.Preview.Branch, DryRun: in.DryRun, TempDir: ex.TempDir,
	})
	if errors.Is(err, ErrBaseMoved) {
		return fail(fmt.Errorf("%w; run it again", err))
	}
	if err != nil {
		return fail(err)
	}
	result.Place, result.Reason = w.Place, w.Reason

	applied, err := Apply{
		Invocation: r.invocation, Release: r.rel, Branch: tp.Branch,
		Message: tp.Preview.CommitMessage, Runner: ex.Runner,
	}.Run(ctx, w)
	if errors.Is(err, ErrNothingChanged) {
		result.Status = ResultUpToDate
		return finish(result, w.Finish(ctx, false))
	}
	if err != nil {
		return fail(errors.Join(err, w.Abort(ctx)))
	}
	result.Commits = applied.Commits
	for _, c := range applied.Changes {
		result.Changes = append(result.Changes, c.Description)
	}
	result.Status = ResultCommitted
	if in.DryRun {
		result.Status = ResultWouldCommit
	}
	return finish(result, w.Finish(ctx, !in.DryRun))
}

// finish records a failure to end the work, which is an error even when the
// commit was made.
func finish(result *TargetResult, err error) *TargetResult {
	if err != nil {
		result.Error = "the work was done, but ending it failed: " + err.Error()
		result.Status = ResultFailed
	}
	return result
}
