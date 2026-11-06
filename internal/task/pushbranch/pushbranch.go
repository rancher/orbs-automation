// Package pushbranch is the task that puts a branch on a remote: one action,
// run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package pushbranch

import (
	"context"
	"errors"
	"fmt"

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

// Run puts the branch on the remote, looking first at what is already there,
// because a plain push of a branch that was pushed before is refused and says
// little. Nothing there is the ordinary push. The same commit needs no push, and
// one the local branch has grown from is an ordinary push too. A branch that has
// gone another way is an earlier attempt's: it is replaced, with a lease on the
// commit that was seen, only when every commit on it is ours and Replace is set.
// Otherwise the error is ErrNotOurs or ErrNeedsReplace, with what is wrong.
func Run(ctx context.Context, g Git, in Input) (Output, error) {
	out := Output{Remote: in.Remote, Branch: in.Branch, Action: ActionPushed}
	plain := func() (Output, error) {
		if err := g.Push(ctx, in.Dir, in.Remote, in.Branch); err != nil {
			return Output{}, fmt.Errorf("push %s to %s: %w", in.Branch, in.Remote, err)
		}
		return out, nil
	}
	tip, err := g.Fetch(ctx, in.Dir, in.Remote, in.Branch)
	if err != nil {
		return plain() // not there, or unreachable, which the push will say
	}
	local, found, err := g.BranchCommit(ctx, in.Dir, in.Branch)
	if err != nil || !found {
		return plain()
	}
	if local == tip {
		out.Action = ActionUpToDate
		return out, nil
	}
	if mergeBase, err := g.MergeBase(ctx, in.Dir, local, tip); err == nil && mergeBase == tip {
		return plain() // the local branch grew from what is there: a fast forward
	}
	base, err := g.MergeBase(ctx, in.Dir, in.Base, tip)
	if err != nil {
		return plain()
	}
	facts, err := g.CommitsSince(ctx, in.Dir, base, tip)
	if err != nil {
		return plain()
	}
	email, err := g.Config(ctx, in.Dir, "user.email")
	if err != nil {
		return plain()
	}
	if ours, why := git.CommitsAreOurs(facts, email); !ours {
		return Output{}, fmt.Errorf("%s on %s: %w: %s", in.Branch, in.Remote, ErrNotOurs, why)
	}
	if !in.Replace {
		return Output{}, fmt.Errorf("%s on %s: %w", in.Branch, in.Remote, ErrNeedsReplace)
	}
	if err := g.PushReplacing(ctx, in.Dir, in.Remote, in.Branch, tip); err != nil {
		return Output{}, fmt.Errorf("replace %s on %s: %w", in.Branch, in.Remote, err)
	}
	out.Action = ActionReplaced
	return out, nil
}
