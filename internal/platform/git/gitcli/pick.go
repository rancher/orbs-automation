package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
// the conflicted files, with the pick in progress. Any other failure leaves nothing in progress.
func CherryPick(ctx context.Context, dir string, p Pick) (string, error) {
	args := append(identityConfig(p.Name, p.Email), "cherry-pick", "-x")
	if p.Mainline > 0 {
		args = append(args, "-m", strconv.Itoa(p.Mainline))
	}
	if p.Sign != nil {
		args = append(args, signFlag(*p.Sign))
	}
	if _, err := inDir(ctx, dir, append(args, p.Commit)...); err != nil {
		files, uerr := Unmerged(ctx, dir)
		if uerr == nil && len(files) > 0 {
			return "", &ConflictError{Commit: p.Commit, Files: files}
		}
		return "", errors.Join(fmt.Errorf("cherry-pick %s: %w", p.Commit, err), abortIfInProgress(ctx, dir))
	}
	return head(ctx, dir)
}

// abortIfInProgress abandons a pick that a failure left half done, and is nil when none was.
func abortIfInProgress(ctx context.Context, dir string) error {
	st, err := Status(ctx, dir)
	if err != nil || st.InProgress != "cherry-pick" {
		return err
	}
	return AbortPick(ctx, dir)
}

func head(ctx context.Context, dir string) (string, error) {
	out, err := inDir(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// Unmerged lists the paths with unresolved conflicts, relative to the top of the work tree.
func Unmerged(ctx context.Context, dir string) ([]string, error) {
	out, err := inDir(ctx, dir, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// ConflictMarkers lists those of files that still hold a conflict marker line. A file that is
// gone, which is how a conflict can be resolved, holds none.
func ConflictMarkers(dir string, files []string) ([]string, error) {
	var left []string
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(nil, 1<<24)
		for sc.Scan() {
			if line := sc.Text(); isMarker(line, "<<<<<<<") || isMarker(line, ">>>>>>>") {
				left = append(left, f)
				break
			}
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
	}
	return left, nil
}

// isMarker reports whether a line is a conflict marker of the given kind: its seven characters,
// alone or followed by a space and a label.
func isMarker(line, marker string) bool {
	return line == marker || strings.HasPrefix(line, marker+" ")
}

// ContinuePick stages the files of a resolved pick and finishes it, keeping its message, and
// returns the new commit. Who commits it is as in Pick.
func ContinuePick(ctx context.Context, dir string, files []string, p Pick) (string, error) {
	if len(files) > 0 {
		if _, err := inDir(ctx, dir, append([]string{"add", "-A", "--"}, files...)...); err != nil {
			return "", err
		}
	}
	args := append(identityConfig(p.Name, p.Email), "-c", "core.editor=true", "cherry-pick", "--continue")
	if p.Sign != nil {
		args = append(args, signFlag(*p.Sign))
	}
	if _, err := inDir(ctx, dir, args...); err != nil {
		return "", err
	}
	return head(ctx, dir)
}

// AbortPick abandons the pick in progress and puts the work tree back as it was before it.
func AbortPick(ctx context.Context, dir string) error {
	_, err := inDir(ctx, dir, "cherry-pick", "--abort")
	return err
}

// ParentCount is how many parents a commit has.
func ParentCount(ctx context.Context, dir, commit string) (int, error) {
	out, err := inDir(ctx, dir, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return 0, err
	}
	return len(strings.Fields(out)) - 1, nil
}

// FirstParentLog lists the n commits that end at tip along first parents, oldest first.
func FirstParentLog(ctx context.Context, dir, tip string, n int) ([]Logged, error) {
	out, err := inDir(ctx, dir, "log", "--first-parent", "--reverse", "-n", strconv.Itoa(n), "--format=%H%x1f%an%x1f%B%x1e", tip)
	if err != nil {
		return nil, err
	}
	var logged []Logged
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.SplitN(record, "\x1f", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git log gave a commit in a shape that was not expected: %q", record)
		}
		logged = append(logged, Logged{Hash: fields[0], Author: fields[1], Message: strings.TrimRight(fields[2], "\n")})
	}
	return logged, nil
}

// HoldsPickOf reports whether the history of ref has a commit that says it was cherry-picked
// from commit, by the line -x writes.
func HoldsPickOf(ctx context.Context, dir, ref, commit string) (bool, error) {
	out, err := inDir(ctx, dir, "log", "-n", "1", "--format=%H", "--fixed-strings", "--grep=(cherry picked from commit "+commit+")", ref)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// IsAncestor reports whether a is an ancestor of b, or the same commit.
func IsAncestor(ctx context.Context, dir, a, b string) (bool, error) {
	_, err := inDir(ctx, dir, "merge-base", "--is-ancestor", a, b)
	switch {
	case err == nil:
		return true, nil
	case exitCode(err) == 1:
		return false, nil
	}
	return false, err
}

// PatchPresent reports whether upstream already has a commit with the same change as commit, by
// patch-id: what git cherry compares. It finds a pick made without -x, or squashed in without its
// line. A merge commit has no patch-id and is never found.
func PatchPresent(ctx context.Context, dir, upstream, commit string) (bool, error) {
	// With the commit's parent as the limit, the only commit on the head side is the commit itself,
	// and it is compared with what upstream has that the commit's history does not.
	out, err := inDir(ctx, dir, "cherry", upstream, commit, commit+"^")
	if err != nil {
		// A root commit has no parent to limit by, and nothing before it to be found in.
		if n, perr := ParentCount(ctx, dir, commit); perr == nil && n == 0 {
			return false, nil
		}
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if sign, hash, ok := strings.Cut(strings.TrimSpace(line), " "); ok && hash == commit {
			return sign == "-", nil
		}
	}
	return false, nil
}
