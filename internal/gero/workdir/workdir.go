// Package workdir is the branch a run edits and commits in: where it is made
// (the engineer's clone or a temporary worktree of it), the commits made in it,
// and putting everything back when the work is over or has failed. Tasks that
// make a branch and tasks that commit to it share it (docs/specs/PROJECT_LAYOUT.md).
package workdir

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// CommitInfo is a commit a run made.
type CommitInfo struct {
	Hash    string `json:"commit"`
	Subject string `json:"subject"`
	// Files are the paths the commit changed, relative to the top of the clone. They
	// are empty when they could not be read, which does not fail the commit.
	Files []string `json:"files,omitempty"`
}

// GitState is what the clone in a directory is in.
type GitState struct {
	Branch     string   // the checked-out branch, "" when HEAD is detached
	InProgress string   // an unfinished merge, rebase, cherry-pick, revert or bisect
	Tracked    []string // tracked files that are staged, modified or deleted
	Untracked  []string // untracked files, one at a time, ignored ones left out
}

// GitCommit is one commit to make.
type GitCommit struct {
	Message     string
	Stage       []string // untracked files the run created, to include
	Name, Email string   // empty for the clone's own git configuration
	Sign        bool
}

// Git is what making a commit in a work directory and ending the work need from
// git. Every operation names the directory it runs in: the engineer's clone, or a
// worktree of it. It is defined here, where it is used.
type Git interface {
	State(ctx context.Context, dir string) (GitState, error)
	Commit(ctx context.Context, dir string, c GitCommit) (commit string, err error)
	// CommitFiles lists the paths a commit changed.
	CommitFiles(ctx context.Context, dir, commit string) ([]string, error)
	Checkout(ctx context.Context, dir, branch string) error
	DeleteBranch(ctx context.Context, dir, branch string) error
	RemoveWorktree(ctx context.Context, dir, path string) error
	Discard(ctx context.Context, dir string, created []string) error
}

// Work is a branch prepared for an edit.
type Work struct {
	// Dir is where to edit and run steps: the clone, or a worktree of it.
	Dir string
	// Place is where the branch was prepared and Reason, for a worktree, why it
	// was not in place.
	Place  Place
	Reason string
	// Reused says the branch already existed, was ours and was current, so it was
	// not built again: Existing are its commits.
	Reused   bool
	Existing []CommitInfo

	g         Git
	clone     string   // the engineer's clone
	branch    string   // the new branch
	start     string   // the branch the clone was on, to go back to in place
	untracked []string // the untracked files the engineer already had, in place
}

// NewInPlace is a branch made in the engineer's own clone, which was on start,
// and had the untracked files it lists before the work began.
func NewInPlace(g Git, clone, branch, start string, untracked []string) *Work {
	return &Work{Dir: clone, Place: InPlace, g: g, clone: clone, branch: branch, start: start, untracked: untracked}
}

// NewInWorktree is a branch made in a temporary worktree of the clone, in dir,
// and the reason it was not made in place.
func NewInWorktree(g Git, clone, dir, branch, reason string) *Work {
	return &Work{Dir: dir, Place: Worktree, Reason: reason, g: g, clone: clone, branch: branch}
}

// NewExisting is a branch of ours that was already there and current, so no work
// was done for it: existing are its commits.
func NewExisting(g Git, clone, branch string, existing []CommitInfo) *Work {
	return &Work{Place: Existing, Reused: true, Existing: existing, g: g, clone: clone, branch: branch}
}

// created lists the untracked files in Dir the run made: the ones that were not
// there when the work began.
func (w *Work) created(ctx context.Context) ([]string, error) {
	st, err := w.g.State(ctx, w.Dir)
	if err != nil {
		return nil, fmt.Errorf("read the state of %s: %w", w.Dir, err)
	}
	return slices.DeleteFunc(st.Untracked, func(p string) bool { return slices.Contains(w.untracked, p) }), nil
}

// Commit commits what the edit and the steps changed in Dir: the tracked files,
// and the untracked ones the run created, never the engineer's own.
func (w *Work) Commit(ctx context.Context, c GitCommit) (string, error) {
	created, err := w.created(ctx)
	if err != nil {
		return "", err
	}
	c.Stage = created
	return w.g.Commit(ctx, w.Dir, c)
}

// CommitFiles lists the paths a commit made in the work directory changed.
func (w *Work) CommitFiles(ctx context.Context, commit string) ([]string, error) {
	return w.g.CommitFiles(ctx, w.Dir, commit)
}

// Finish ends the work. In place the clone goes back to the branch it started
// on; a worktree is removed. The new branch is kept when keep is set, and
// deleted otherwise. Every step is tried, and the errors are joined.
func (w *Work) Finish(ctx context.Context, keep bool) error {
	if w.Reused {
		return nil // the branch was there before, and is not ours to remove now
	}
	var errs []error
	if w.Place == InPlace {
		errs = append(errs, w.g.Checkout(ctx, w.clone, w.start))
	} else {
		errs = append(errs, w.g.RemoveWorktree(ctx, w.clone, w.Dir))
	}
	if !keep {
		errs = append(errs, w.g.DeleteBranch(ctx, w.clone, w.branch))
	}
	return errors.Join(errs...)
}

// Abort ends the work after a failure: whatever the run changed in the clone is
// undone, and the branch is not kept.
func (w *Work) Abort(ctx context.Context) error {
	if w.Reused {
		return nil
	}
	var errs []error
	if w.Place == InPlace {
		created, err := w.created(ctx)
		if err == nil {
			err = w.g.Discard(ctx, w.Dir, created)
		}
		errs = append(errs, err)
	}
	return errors.Join(append(errs, w.Finish(ctx, false))...)
}
