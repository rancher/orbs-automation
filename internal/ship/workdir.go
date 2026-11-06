package ship

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/git/gitcli"
)

var (
	// ErrBaseMoved is returned by Prepare when the branch fetched from the
	// remote is not at the commit the state check read: the check has to be read
	// again before anything is written.
	ErrBaseMoved = errors.New("the target branch moved since it was checked")
	// ErrBranchExists is returned by Prepare when the new branch's name is taken
	// by a branch that holds commits of its own, which are the engineer's to keep.
	ErrBranchExists = errors.New("the branch already exists with commits of its own")
	// ErrBranchCheckedOut is returned by Prepare when the new branch's name is
	// the branch the clone has checked out, so it can be neither replaced nor
	// moved away from.
	ErrBranchCheckedOut = errors.New("the branch is checked out in the clone")
)

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

// Git is what preparing, committing in and cleaning up a work directory needs
// from git. Every operation names the directory it runs in: the engineer's
// clone, or a worktree of it. It is defined here, where it is used, so a fake
// can stand in for it, and GitCLI is the implementation.
type Git interface {
	Fetch(ctx context.Context, dir, remote, branch string) (commit string, err error)
	State(ctx context.Context, dir string) (GitState, error)
	Collisions(ctx context.Context, dir, commit string, untracked []string) ([]string, error)
	BranchCommit(ctx context.Context, dir, branch string) (commit string, found bool, err error)
	StartBranch(ctx context.Context, dir, branch, commit string) error
	Checkout(ctx context.Context, dir, branch string) error
	DeleteBranch(ctx context.Context, dir, branch string) error
	AddWorktree(ctx context.Context, dir, path, branch, commit string) error
	RemoveWorktree(ctx context.Context, dir, path string) error
	Commit(ctx context.Context, dir string, c GitCommit) (commit string, err error)
	Discard(ctx context.Context, dir string, created []string) error
	Push(ctx context.Context, dir, remote, branch string) error
}

// GitCLI is Git through the git binary (internal/git/gitcli).
type GitCLI struct{}

var _ Git = GitCLI{}

func (GitCLI) Fetch(ctx context.Context, dir, remote, branch string) (string, error) {
	return gitcli.Fetch(ctx, dir, remote, branch)
}

func (GitCLI) State(ctx context.Context, dir string) (GitState, error) {
	st, err := gitcli.Status(ctx, dir)
	return GitState{Branch: st.Branch, InProgress: st.InProgress, Tracked: st.Tracked, Untracked: st.Untracked}, err
}

func (GitCLI) Collisions(ctx context.Context, dir, commit string, untracked []string) ([]string, error) {
	return gitcli.Collisions(ctx, dir, commit, untracked)
}

func (GitCLI) BranchCommit(ctx context.Context, dir, branch string) (string, bool, error) {
	return gitcli.BranchCommit(ctx, dir, branch)
}

func (GitCLI) StartBranch(ctx context.Context, dir, branch, commit string) error {
	return gitcli.StartBranch(ctx, dir, branch, commit)
}

func (GitCLI) Checkout(ctx context.Context, dir, branch string) error {
	return gitcli.Checkout(ctx, dir, branch)
}

func (GitCLI) DeleteBranch(ctx context.Context, dir, branch string) error {
	return gitcli.DeleteBranch(ctx, dir, branch)
}

func (GitCLI) AddWorktree(ctx context.Context, dir, path, branch, commit string) error {
	return gitcli.AddWorktree(ctx, dir, path, branch, commit)
}

func (GitCLI) RemoveWorktree(ctx context.Context, dir, path string) error {
	return gitcli.RemoveWorktree(ctx, dir, path)
}

func (GitCLI) Commit(ctx context.Context, dir string, c GitCommit) (string, error) {
	return gitcli.CommitChanges(ctx, dir, gitcli.Commit{
		Message: c.Message, Stage: c.Stage, Name: c.Name, Email: c.Email, Sign: c.Sign,
	})
}

func (GitCLI) Push(ctx context.Context, dir, remote, branch string) error {
	return gitcli.Push(ctx, dir, remote, branch)
}

func (GitCLI) Discard(ctx context.Context, dir string, created []string) error {
	return gitcli.Discard(ctx, dir, created)
}

// PrepareRequest says which branch to prepare, and from what.
type PrepareRequest struct {
	CloneDir string // the engineer's clone of the consumer
	Remote   string // the remote the target branch is fetched from
	Target   string // the consumer branch the bump goes to, such as release/v2.15
	Base     string // the commit of Target the state check read
	Branch   string // the new branch to create
	DryRun   bool   // never move the checkout
	TempDir  string // where a worktree goes
}

// Work is a branch prepared for an edit.
type Work struct {
	// Dir is where to edit and run steps: the clone, or a worktree of it.
	Dir string
	// Place is where the branch was prepared and Reason, for a worktree, why it
	// was not in place.
	Place  Place
	Reason string

	g         Git
	clone     string   // the engineer's clone
	branch    string   // the new branch
	start     string   // the branch the clone was on, to go back to in place
	untracked []string // the untracked files the engineer already had, in place
}

// Prepare fetches the target branch and creates the new branch at the commit
// the state check read, in place or in a worktree as ChoosePlace says. It
// creates nothing when it fails.
func Prepare(ctx context.Context, g Git, req PrepareRequest) (*Work, error) {
	st, err := g.State(ctx, req.CloneDir)
	if err != nil {
		return nil, fmt.Errorf("read the state of %s: %w", req.CloneDir, err)
	}
	head, err := g.Fetch(ctx, req.CloneDir, req.Remote, req.Target)
	if err != nil {
		return nil, err
	}
	if head != req.Base {
		return nil, fmt.Errorf("%w: %s is at %s, and it was checked at %s", ErrBaseMoved, req.Target, short(head), short(req.Base))
	}
	if st.Branch == req.Branch {
		return nil, fmt.Errorf("%w: %s", ErrBranchCheckedOut, req.Branch)
	}
	if commit, found, err := g.BranchCommit(ctx, req.CloneDir, req.Branch); err != nil {
		return nil, err
	} else if found {
		if commit != req.Base {
			return nil, fmt.Errorf("%w: %s is at %s, not at %s: delete it or keep it, it is yours", ErrBranchExists, req.Branch, short(commit), short(req.Base))
		}
		// Identical to where the new branch would start, so nothing is lost.
		if err := g.DeleteBranch(ctx, req.CloneDir, req.Branch); err != nil {
			return nil, err
		}
	}
	collisions, err := g.Collisions(ctx, req.CloneDir, req.Base, st.Untracked)
	if err != nil {
		return nil, err
	}
	choice := ChoosePlace(CloneState{Branch: st.Branch, InProgress: st.InProgress, Tracked: st.Tracked, Collisions: collisions}, req.DryRun)
	w := &Work{Place: choice.Place, Reason: choice.Reason, g: g, clone: req.CloneDir, branch: req.Branch}
	switch choice.Place {
	case InPlace:
		if err := g.StartBranch(ctx, req.CloneDir, req.Branch, req.Base); err != nil {
			return nil, err
		}
		w.Dir, w.start, w.untracked = req.CloneDir, st.Branch, st.Untracked
	default:
		w.Dir = filepath.Join(req.TempDir, strings.ReplaceAll(req.Branch, "/", "-"))
		if err := g.AddWorktree(ctx, req.CloneDir, w.Dir, req.Branch, req.Base); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// short abbreviates a commit for a message.
func short(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
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

// Finish ends the work. In place the clone goes back to the branch it started
// on; a worktree is removed. The new branch is kept when keep is set, and
// deleted otherwise. Every step is tried, and the errors are joined.
func (w *Work) Finish(ctx context.Context, keep bool) error {
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
