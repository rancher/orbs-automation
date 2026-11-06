package gitcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/execx"
)

// The functions in this file prepare a branch of a local clone, change it and
// commit, for gero ship. Each is the git command the push-to-rancher script
// (scc-operator's create-prs.sh) runs for the same step, run the same way
// (git -C <dir> ...), with the differences that script's own steps need
// recorded where they are. Why not go-git: it cannot create linked worktrees or
// switch a checkout the way git does, and the commit must run the engineer's
// hooks and signing settings, which only git applies.

// ErrNothingToCommit is returned by Commit when staging leaves no change.
var ErrNothingToCommit = errors.New("nothing to commit")

// inDir runs git in dir and returns stdout. A failure carries the last of what
// git said, since that is where the cause is.
func inDir(ctx context.Context, dir string, args ...string) (string, error) {
	res, err := runGit(ctx, append([]string{"-C", dir}, args...)...)
	if err != nil {
		var exit *execx.ExitError
		if errors.As(err, &exit) {
			if msg := strings.TrimSpace(string(res.Stderr)); msg != "" {
				return "", fmt.Errorf("git %s: %w: %s", args[0], err, lastLine(msg))
			}
		}
		return "", wrap(err, "git "+args[0])
	}
	return string(res.Stdout), nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// exitCode returns the exit code of a git failure, or -1 for anything else.
func exitCode(err error) int {
	var exit *execx.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode
	}
	return -1
}

// Fetch fetches one branch of remote into the clone in dir and returns the
// commit it is at. This is the script's `git fetch $RANCHER_REMOTE $BRANCH`:
// it adds objects and updates remote/branch, and moves no local branch and
// nothing in the working tree.
func Fetch(ctx context.Context, dir, remote, branch string) (string, error) {
	if _, err := inDir(ctx, dir, "fetch", remote, branch); err != nil {
		return "", err
	}
	out, err := inDir(ctx, dir, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HasCommit reports whether the clone holds the commit.
func HasCommit(ctx context.Context, dir, commit string) (bool, error) {
	_, err := inDir(ctx, dir, "cat-file", "-e", commit+"^{commit}")
	if exitCode(err) == 1 || exitCode(err) == 128 {
		return false, nil
	}
	return err == nil, err
}

// BranchCommit returns the commit a local branch is at, and false when there is
// no such branch.
func BranchCommit(ctx context.Context, dir, branch string) (string, bool, error) {
	out, err := inDir(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch+"^{commit}")
	if exitCode(err) == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

// CountCommits returns how many commits are reachable from to and not from
// from (`git rev-list --count from..to`). It is false when the clone lacks
// either commit, so the answer is unknown and not zero.
func CountCommits(ctx context.Context, dir, from, to string) (int, bool, error) {
	for _, c := range []string{from, to} {
		has, err := HasCommit(ctx, dir, c)
		if err != nil || !has {
			return 0, false, err
		}
	}
	out, err := inDir(ctx, dir, "rev-list", "--count", from+".."+to)
	if err != nil {
		return 0, false, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, false, fmt.Errorf("git rev-list: count %q: %w", strings.TrimSpace(out), err)
	}
	return n, true, nil
}

// State is what a clone is in, as far as starting a branch in it is concerned.
type State struct {
	// Branch is the checked-out branch, or "" when HEAD is detached.
	Branch string
	// InProgress names an unfinished operation (merge, rebase, cherry-pick,
	// revert, bisect), or is "".
	InProgress string
	// Tracked lists tracked paths that are staged, modified or deleted.
	Tracked []string
	// Untracked lists paths git does not track and does not ignore, a file at a
	// time.
	Untracked []string
}

// Clean reports whether the work tree has no change to a tracked file and no
// operation in progress. Untracked files do not make it unclean (Collisions says
// when one still gets in the way), and neither does a detached HEAD.
func (s State) Clean() bool {
	return s.InProgress == "" && len(s.Tracked) == 0
}

// CleanOnBranch reports whether the clone can be worked in directly: it is
// Clean and HEAD is a branch, since a branch cannot be started in place from a
// detached HEAD without losing where it was.
func (s State) CleanOnBranch() bool { return s.Branch != "" && s.Clean() }

// inProgress maps the file git leaves in its directory to the operation.
var inProgress = []struct{ file, name string }{
	{"MERGE_HEAD", "merge"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"},
	{"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"}, {"BISECT_LOG", "bisect"},
}

// Status reads the state of the clone in dir. It is the script's `git diff
// --quiet` and `git status --porcelain` checks, kept apart: what is tracked and
// changed, and what is untracked.
func Status(ctx context.Context, dir string) (State, error) {
	var st State
	out, err := inDir(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	switch {
	case err == nil:
		st.Branch = strings.TrimSpace(out)
	case exitCode(err) != 1: // 1 is a detached HEAD
		return State{}, err
	}
	porcelain, err := inDir(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return State{}, err
	}
	fields := strings.Split(porcelain, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		xy, path := f[:2], f[3:]
		if xy == "??" {
			st.Untracked = append(st.Untracked, path)
			continue
		}
		st.Tracked = append(st.Tracked, path)
		if xy[0] == 'R' || xy[0] == 'C' || xy[1] == 'R' || xy[1] == 'C' {
			i++ // a rename or copy is followed by the path it came from
		}
	}
	for _, p := range inProgress {
		gitPath, err := inDir(ctx, dir, "rev-parse", "--git-path", p.file)
		if err != nil {
			return State{}, err
		}
		gitPath = strings.TrimSpace(gitPath)
		if !filepath.IsAbs(gitPath) {
			gitPath = filepath.Join(dir, gitPath)
		}
		if _, err := os.Stat(gitPath); err == nil {
			st.InProgress = p.name
			break
		}
	}
	return st, nil
}

// Collisions returns the paths of untracked that checking out commit would
// overwrite: the commit has a file at that path, or anything below it (the path
// would have to be a directory), or has a file where one of its parent
// directories would be. git refuses such a checkout, so the caller works
// elsewhere.
func Collisions(ctx context.Context, dir, commit string, untracked []string) ([]string, error) {
	var collisions []string
	for _, path := range untracked {
		hit, err := collides(ctx, dir, commit, path)
		if err != nil {
			return nil, err
		}
		if hit {
			collisions = append(collisions, path)
		}
	}
	return collisions, nil
}

func collides(ctx context.Context, dir, commit, path string) (bool, error) {
	out, err := inDir(ctx, dir, "ls-tree", "-r", "--name-only", commit, "--", path)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(out) != "" {
		return true, nil
	}
	for parent := filepath.Dir(path); parent != "." && parent != "/"; parent = filepath.Dir(parent) {
		out, err := inDir(ctx, dir, "ls-tree", commit, "--", parent)
		if err != nil {
			return false, err
		}
		for _, line := range strings.Split(out, "\n") {
			if fields := strings.Fields(line); len(fields) >= 2 && fields[1] != "tree" {
				return true, nil // a file where the directory would be
			}
		}
	}
	return false, nil
}

// StartBranch creates branch at commit and checks it out in the clone in dir:
// the script's `git checkout -b $BRANCH_NAME`, but at the commit that was
// fetched and checked, never by first resetting a local branch to it (the
// script's `checkout -B`). An existing branch is an error.
func StartBranch(ctx context.Context, dir, branch, commit string) error {
	_, err := inDir(ctx, dir, "checkout", "--quiet", "-b", branch, commit)
	return err
}

// Checkout checks out an existing branch, as the script's `git checkout
// $TARGET_BRANCH` does to return, but without its -f: nothing is discarded.
func Checkout(ctx context.Context, dir, branch string) error {
	_, err := inDir(ctx, dir, "checkout", "--quiet", branch)
	return err
}

// DeleteBranch deletes a local branch whatever it holds: the script's
// `git branch -D $BRANCH_NAME`.
func DeleteBranch(ctx context.Context, dir, branch string) error {
	_, err := inDir(ctx, dir, "branch", "--quiet", "-D", branch)
	return err
}

// AddWorktree creates a worktree of the clone in dir at path, on a new branch at
// commit. The script has no equivalent: it edits the engineer's checkout.
func AddWorktree(ctx context.Context, dir, path, branch, commit string) error {
	_, err := inDir(ctx, dir, "worktree", "add", "--quiet", "-b", branch, path, commit)
	return err
}

// RemoveWorktree removes a worktree made by AddWorktree, with whatever it holds.
func RemoveWorktree(ctx context.Context, dir, path string) error {
	_, err := inDir(ctx, dir, "worktree", "remove", "--force", path)
	return err
}

// Commit is one commit to make.
type Commit struct {
	// Message is the whole commit message.
	Message string
	// Stage lists untracked paths the run created and that belong in the commit.
	// Tracked changes are always staged, and no other untracked path is.
	Stage []string
	// Name and Email are the author and committer, or both empty for the git
	// configuration of the clone, which is what the script's local runs use.
	Name, Email string
	// Sign makes git sign the commit with the engineer's own configuration.
	// Otherwise the commit is unsigned whatever the configuration says.
	Sign bool
}

// CommitChanges stages and commits in the clone in dir, and returns the new
// commit. The script's `git add .` and `git commit -m` differ in what they
// stage: `add .` takes every untracked file in the checkout, including the
// engineer's own, so here it is the tracked changes (`add -u`) and the paths in
// Stage. The commit runs hooks, as the script's does. It is ErrNothingToCommit
// when staging leaves no change.
func CommitChanges(ctx context.Context, dir string, c Commit) (string, error) {
	if _, err := inDir(ctx, dir, "add", "-u"); err != nil {
		return "", err
	}
	if len(c.Stage) > 0 {
		if _, err := inDir(ctx, dir, append([]string{"add", "--"}, c.Stage...)...); err != nil {
			return "", err
		}
	}
	_, err := inDir(ctx, dir, "diff", "--cached", "--quiet")
	if err == nil {
		return "", ErrNothingToCommit
	}
	if exitCode(err) != 1 {
		return "", err
	}
	var args []string
	if c.Name != "" || c.Email != "" {
		args = append(args, "-c", "user.name="+c.Name, "-c", "user.email="+c.Email)
	}
	args = append(args, "commit", "--quiet", "-m", c.Message)
	if c.Sign {
		args = append(args, "--gpg-sign")
	} else {
		args = append(args, "--no-gpg-sign")
	}
	if _, err := inDir(ctx, dir, args...); err != nil {
		return "", err
	}
	out, err := inDir(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// Discard undoes a run in a clone that was clean when it started: tracked files
// go back to HEAD, and the untracked paths the run created are removed. It is
// what the script's `git checkout -f` is for, narrowed to what the run made.
func Discard(ctx context.Context, dir string, created []string) error {
	if _, err := inDir(ctx, dir, "reset", "--hard", "--quiet", "HEAD"); err != nil {
		return err
	}
	if len(created) == 0 {
		return nil
	}
	_, err := inDir(ctx, dir, append([]string{"clean", "--force", "--quiet", "--"}, created...)...)
	return err
}

// CommitFact is what a commit says about who made it and what it is.
type CommitFact struct {
	Hash        string
	AuthorEmail string
	Message     string // the whole message, with its trailers
}

// MergeBase returns the best common ancestor of two commits: where a branch
// left the branch it was made from. Commits with no common ancestor are an
// error.
func MergeBase(ctx context.Context, dir, a, b string) (string, error) {
	out, err := inDir(ctx, dir, "merge-base", a, b)
	if exitCode(err) == 1 {
		return "", fmt.Errorf("%s and %s have no common ancestor", a, b)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitsSince lists the commits reachable from tip and not from base, oldest
// first: what a branch added.
func CommitsSince(ctx context.Context, dir, base, tip string) ([]CommitFact, error) {
	out, err := inDir(ctx, dir, "log", "--reverse", "--format=%H%x1f%ae%x1f%B%x1e", base+".."+tip)
	if err != nil {
		return nil, err
	}
	var facts []CommitFact
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.SplitN(record, "\x1f", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git log gave a commit in a shape that was not expected: %q", record)
		}
		facts = append(facts, CommitFact{Hash: fields[0], AuthorEmail: fields[1], Message: strings.TrimRight(fields[2], "\n")})
	}
	return facts, nil
}

// ChangeFingerprint identifies the change tip makes to base, so two branches can
// be compared by what they do and not by the trees they sit on, which differ
// wherever the branch they came from has moved. It is a hash of the diff with no
// context lines, and without what depends on where the change is or on what
// surrounds it: the blob ids and the positions of the hunks.
func ChangeFingerprint(ctx context.Context, dir, base, tip string) (string, error) {
	out, err := inDir(ctx, dir, "diff", "--no-color", "--no-ext-diff", "--no-renames", "-U0", base, tip)
	if err != nil {
		return "", err
	}
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "index "):
			continue
		case strings.HasPrefix(line, "@@"):
			line = "@@"
		}
		kept = append(kept, line)
	}
	sum := sha256.Sum256([]byte(strings.Join(kept, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// ConfigIn returns what git would use for key in the clone in dir, or "" when it
// is not set: the clone's own settings, then the user's and the system's. It is
// how to know the identity a commit made there would have.
func ConfigIn(ctx context.Context, dir, key string) (string, error) {
	out, err := inDir(ctx, dir, "config", "--get", key)
	if exitCode(err) == 1 {
		return "", nil // not set
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ForceBranch makes the branch name point at commit, creating it or moving it,
// whatever it held. It is `git branch -f`, which refuses for the branch that is
// checked out, so a working tree is never changed under anyone.
//
// STUB: does nothing until a later commit.
func ForceBranch(ctx context.Context, dir, name, commit string) error { return nil }
