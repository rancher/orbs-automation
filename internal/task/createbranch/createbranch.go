// Package createbranch is the task that makes a branch off the upstream branch
// of a project's release line, ready to be edited and committed to: one action,
// run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package createbranch

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workdir"
)

var (
	// ErrBaseMoved is returned when the branch fetched from the remote is not at
	// the commit the caller read: whatever was decided from it has to be read
	// again before anything is written.
	ErrBaseMoved = errors.New("the target branch moved since it was checked")
	// ErrBranchExists is returned when the new branch's name is taken by a branch
	// that holds commits of its own, which are the engineer's to keep.
	ErrBranchExists = errors.New("the branch already exists with commits of its own")
	// ErrBranchCheckedOut is returned when the new branch's name is the branch
	// the clone has checked out, so it can be neither replaced nor moved away from.
	ErrBranchCheckedOut = errors.New("the branch is checked out in the clone")
)

// Input says which branch to make, and from what.
type Input struct {
	CloneDir string // the engineer's clone of the repository
	Remote   string // the remote the target branch is fetched from
	Target   string // the branch the new one is made from, such as release/v2.15
	Base     string // the commit of Target the caller read
	Branch   string // the new branch to create
	DryRun   bool   // never move the checkout
	TempDir  string // where a worktree goes
	// Rebuild makes the branch again even when one of ours is there and current.
	Rebuild bool
}

// Output is the branch that was made. Work is a handle on it, to commit to and to
// end: it is shared with the tasks that go on from here, through
// internal/workdir, and is not meant to be serialized.
type Output struct {
	Work *workdir.Work
}

// Git is what making the branch needs from git, with what the work needs to be
// ended. It is defined here, where it is used.
type Git interface {
	workdir.Git
	Fetch(ctx context.Context, dir, remote, branch string) (commit string, err error)
	Collisions(ctx context.Context, dir, commit string, untracked []string) ([]string, error)
	BranchCommit(ctx context.Context, dir, branch string) (commit string, found bool, err error)
	StartBranch(ctx context.Context, dir, branch, commit string) error
	AddWorktree(ctx context.Context, dir, path, branch, commit string) error
	MergeBase(ctx context.Context, dir, a, b string) (string, error)
	CommitsSince(ctx context.Context, dir, base, tip string) ([]git.BranchCommit, error)
	// Config is what git uses for key in the clone: user.email is the actor's.
	Config(ctx context.Context, dir, key string) (string, error)
}

// Run makes the branch. TODO: the stub does nothing yet.
func Run(_ context.Context, _ Git, _ Input) (Output, error) {
	return Output{}, nil
}
