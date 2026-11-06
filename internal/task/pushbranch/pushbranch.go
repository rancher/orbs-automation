// Package pushbranch is the task that puts a branch on a remote: one action,
// run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package pushbranch

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/git"
)

// Action is what Run did.
type Action string

const (
	// ActionPushed: the branch was not on the remote, or the remote's was an
	// ancestor of it, and an ordinary push put it there.
	ActionPushed Action = "pushed"
	// ActionUpToDate: the remote already had this commit, so nothing was sent.
	ActionUpToDate Action = "up to date"
	// ActionReplaced: the remote had commits of ours from an earlier attempt, and
	// they were replaced.
	ActionReplaced Action = "replaced"
)

var (
	// ErrNeedsReplace is returned when the remote has different commits of ours
	// from an earlier attempt and Replace was not set.
	ErrNeedsReplace = errors.New("the branch on the remote already has different commits of ours, from an earlier attempt")
	// ErrNotOurs is returned when the remote's copy has a commit that is not
	// ours, which is never replaced.
	ErrNotOurs = errors.New("the branch on the remote has commits that are not ours, so it is not replaced")
)

// Input is what to push, and where.
type Input struct {
	Dir    string // the clone, or a worktree of it, the branch is in
	Remote string // the remote's name
	Branch string
	// Base is the commit the branch was built on, to tell which commits on the
	// remote's copy are an earlier attempt's.
	Base string
	// Replace lets a branch of ours that is already on the remote, with commits
	// that are not the ones being pushed, be replaced.
	Replace bool
}

// Output is what was done.
type Output struct {
	Remote string
	Branch string
	Action Action
}

// Git is what the push needs from git. It is defined here, where it is used.
type Git interface {
	Fetch(ctx context.Context, dir, remote, branch string) (commit string, err error)
	BranchCommit(ctx context.Context, dir, branch string) (commit string, found bool, err error)
	MergeBase(ctx context.Context, dir, a, b string) (string, error)
	CommitsSince(ctx context.Context, dir, base, tip string) ([]git.BranchCommit, error)
	// Config is what git uses for key in the clone: user.email is the actor's.
	Config(ctx context.Context, dir, key string) (string, error)
	Push(ctx context.Context, dir, remote, branch string) error
	// PushReplacing pushes over a branch that is at expected on the remote.
	PushReplacing(ctx context.Context, dir, remote, branch, expected string) error
}

// Run pushes the branch. TODO: the stub does nothing yet.
func Run(_ context.Context, _ Git, _ Input) (Output, error) {
	return Output{}, nil
}
