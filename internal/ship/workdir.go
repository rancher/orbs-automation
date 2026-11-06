package ship

import (
	"context"
	"errors"

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
}

// Prepare fetches the target branch and creates the new branch at the commit
// the state check read, in place or in a worktree as ChoosePlace says.
//
// STUB: prepares nothing until a later commit.
func Prepare(ctx context.Context, g Git, req PrepareRequest) (*Work, error) {
	return &Work{}, nil
}

// Commit commits what the edit and the steps changed in Dir: the tracked files,
// and the untracked ones the run created, never the engineer's own.
//
// STUB: commits nothing until a later commit.
func (w *Work) Commit(ctx context.Context, g Git, c GitCommit) (string, error) {
	return "", nil
}

// Finish ends the work. In place the clone goes back to the branch it started
// on; a worktree is removed. The new branch is kept when keep is set, and
// deleted otherwise.
//
// STUB: does nothing until a later commit.
func (w *Work) Finish(ctx context.Context, g Git, keep bool) error { return nil }

// Abort ends the work after a failure: whatever the run changed in the clone is
// undone, and the branch is not kept.
//
// STUB: does nothing until a later commit.
func (w *Work) Abort(ctx context.Context, g Git) error { return nil }
