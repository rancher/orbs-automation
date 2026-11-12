// Package createbranch is the task that makes a branch off the upstream branch
// of a project's release line, ready to be edited and committed to: one action,
// run alone or as a step of a longer process (docs/specs/PROJECT_LAYOUT.md,
// Tasks).
package createbranch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/workdir"
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
	// StageUntracked commits every untracked file in the checkout when the work is
	// done in place, the ones that were there before the run too. Without it the
	// engineer's own untracked files are left out. A run that owns its checkout,
	// such as one in CI, sets it.
	StageUntracked bool
}

// Output is the branch that was made. Work is a handle on it, to commit to and to
// end: it is shared with the tasks that go on from here, through
// internal/platform/workdir, and is not meant to be serialized.
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

// Run fetches the target branch and makes the new branch at the commit the
// caller read, in place or in a worktree as workdir.ChoosePlace says. A branch of
// the name that is already there is settled first (see settleExisting). It makes
// nothing when it fails.
func Run(ctx context.Context, g Git, in Input) (Output, error) {
	st, err := g.State(ctx, in.CloneDir)
	if err != nil {
		return Output{}, fmt.Errorf("read the state of %s: %w", in.CloneDir, err)
	}
	head, err := g.Fetch(ctx, in.CloneDir, in.Remote, in.Target)
	if err != nil {
		return Output{}, err
	}
	if head != in.Base {
		return Output{}, fmt.Errorf("%w: %s is at %s, and it was checked at %s", ErrBaseMoved, in.Target, shortHash(head), shortHash(in.Base))
	}
	if st.Branch == in.Branch {
		return Output{}, fmt.Errorf("%w: %s", ErrBranchCheckedOut, in.Branch)
	}
	if commit, found, err := g.BranchCommit(ctx, in.CloneDir, in.Branch); err != nil {
		return Output{}, err
	} else if found {
		reused, err := settleExisting(ctx, g, in, commit)
		if err != nil {
			return Output{}, err
		}
		if reused != nil {
			return Output{Work: reused}, nil
		}
	}
	collisions, err := g.Collisions(ctx, in.CloneDir, in.Base, st.Untracked)
	if err != nil {
		return Output{}, err
	}
	choice := workdir.ChoosePlace(workdir.CloneState{Branch: st.Branch, InProgress: st.InProgress, Tracked: st.Tracked, Collisions: collisions}, in.DryRun)
	var w *workdir.Work
	switch choice.Place {
	case workdir.InPlace:
		if err := g.StartBranch(ctx, in.CloneDir, in.Branch, in.Base); err != nil {
			return Output{}, err
		}
		w = workdir.NewInPlace(g, in.CloneDir, in.Branch, st.Branch, st.Untracked)
	default:
		dir := filepath.Join(in.TempDir, strings.ReplaceAll(in.Branch, "/", "-"))
		if err := g.AddWorktree(ctx, in.CloneDir, dir, in.Branch, in.Base); err != nil {
			return Output{}, err
		}
		w = workdir.NewInWorktree(g, in.CloneDir, dir, in.Branch, choice.Reason)
	}
	return Output{Work: w}, nil
}

// settleExisting decides what to do about a branch of the name that is already
// there, at commit. One that is at the base itself holds nothing, and goes. One
// whose commits are all ours (see git.CommitsAreOurs) is the actor's own earlier work:
// when it is based on the current head of the target it is reused as it is, which
// is also how a run whose push failed is resumed, and otherwise, or when a rebuild
// was asked for, it goes and is built again. One with a commit that is not ours is
// an error, since those commits are somebody's to keep. A nil Work with no error
// says to go on and build.
func settleExisting(ctx context.Context, g Git, in Input, commit string) (*workdir.Work, error) {
	if commit != in.Base {
		mergeBase, err := g.MergeBase(ctx, in.CloneDir, in.Base, commit)
		if err != nil {
			return nil, fmt.Errorf("compare the branch %s with %s: %w", in.Branch, in.Target, err)
		}
		facts, err := g.CommitsSince(ctx, in.CloneDir, mergeBase, commit)
		if err != nil {
			return nil, err
		}
		email, err := g.Config(ctx, in.CloneDir, "user.email")
		if err != nil {
			return nil, err
		}
		if ours, why := git.CommitsAreOurs(facts, email); !ours {
			return nil, fmt.Errorf("%w: %s is at %s, and %s: delete it or keep it, it is not ours to change", ErrBranchExists, in.Branch, shortHash(commit), why)
		}
		if mergeBase == in.Base && !in.Rebuild {
			existing := make([]workdir.CommitInfo, len(facts))
			for i, f := range facts {
				subject, _, _ := strings.Cut(f.Message, "\n")
				existing[i] = workdir.CommitInfo{Hash: f.Hash, Subject: subject}
				if files, err := g.CommitFiles(ctx, in.CloneDir, f.Hash); err == nil {
					existing[i].Files = files // for the report, as for a commit just made
				}
			}
			return workdir.NewExisting(g, in.CloneDir, in.Branch, existing), nil
		}
	}
	// At the base, or ours and out of date, or to be rebuilt: nothing is lost.
	if err := g.DeleteBranch(ctx, in.CloneDir, in.Branch); err != nil {
		return nil, err
	}
	return nil, nil
}

// shortHash abbreviates a commit for a message.
func shortHash(commit string) string {
	return commit[:min(len(commit), 8)]
}
