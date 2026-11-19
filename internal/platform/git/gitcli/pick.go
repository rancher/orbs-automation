package gitcli

import (
	"context"
	"errors"
)

// The functions in this file are what a backport does with git: cherry-pick a commit with its
// origin recorded, stop on a conflict with the pick left in progress, and finish or abandon
// it. Each runs git in dir, so it works in the engineer's clone or in a worktree of it.

// ErrConflict is wrapped by CherryPick when the pick stopped on conflicts. The pick is left in
// progress in the work tree: ContinuePick finishes it and AbortPick abandons it.
var ErrConflict = errors.New("cherry-pick stopped on conflicts")

// ConflictError is the error of a pick that stopped on conflicts, and names the files.
type ConflictError struct {
	Commit string
	Files  []string
}

func (e *ConflictError) Error() string { return "cherry-pick of " + e.Commit + " conflicts" }
func (e *ConflictError) Unwrap() error { return ErrConflict }

// Pick is one cherry-pick to make.
type Pick struct {
	Commit string
	// Mainline is the parent to take the change against when Commit is a merge, or 0.
	Mainline int
	// Name and Email are who commits the pick, or both empty for the git configuration of the
	// clone. The author of the picked commit is always kept.
	Name, Email string
	// Sign says whether git signs the commit; nil leaves it to git (commit.gpgSign).
	Sign *bool
}

// Logged is a commit as git log reports it.
type Logged struct {
	Hash    string
	Author  string // the author's name
	Message string // the whole message
}

// CherryPick picks a commit onto HEAD with -x, so its message records where it came from, and
// returns the new commit. When the pick conflicts it is an *ConflictError (ErrConflict) naming
// the conflicted files, with the pick in progress.
func CherryPick(ctx context.Context, dir string, p Pick) (string, error) {
	return "", nil
}

// Unmerged lists the paths with unresolved conflicts, relative to the top of the work tree.
func Unmerged(ctx context.Context, dir string) ([]string, error) { return nil, nil }

// ConflictMarkers lists those of files that still hold a conflict marker line.
func ConflictMarkers(dir string, files []string) ([]string, error) { return nil, nil }

// ContinuePick stages the files of a resolved pick and finishes it, keeping its message, and
// returns the new commit. Who commits it is as in Pick.
func ContinuePick(ctx context.Context, dir string, files []string, p Pick) (string, error) {
	return "", nil
}

// AbortPick abandons the pick in progress and puts the work tree back as it was before it.
func AbortPick(ctx context.Context, dir string) error { return nil }

// ParentCount is how many parents a commit has.
func ParentCount(ctx context.Context, dir, commit string) (int, error) { return 0, nil }

// FirstParentLog lists the n commits that end at tip along first parents, oldest first.
func FirstParentLog(ctx context.Context, dir, tip string, n int) ([]Logged, error) { return nil, nil }

// HoldsPickOf reports whether the history of ref has a commit that says it was cherry-picked
// from commit, by the line -x writes.
func HoldsPickOf(ctx context.Context, dir, ref, commit string) (bool, error) { return false, nil }
