package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/rancher/orbs-automation/internal/task/createbranch"
	"github.com/rancher/orbs-automation/internal/workdir"
)

// The work directory a run edits and commits in lives in internal/workdir.
type (
	// Work is a branch prepared for an edit.
	Work = workdir.Work
	// Place is where the branch for one target is prepared.
	Place = workdir.Place
	// GitState is what the clone in a directory is in.
	GitState = workdir.GitState
	// GitCommit is one commit to make.
	GitCommit = workdir.GitCommit
)

const (
	InPlace  = workdir.InPlace
	Worktree = workdir.Worktree
	Existing = workdir.Existing
)

// BranchCommit is a commit on a branch, as far as telling whose it is.
type BranchCommit = git.BranchCommit

// What went wrong in making a branch, as the createbranch task says it.
var (
	ErrBaseMoved        = createbranch.ErrBaseMoved
	ErrBranchExists     = createbranch.ErrBranchExists
	ErrBranchCheckedOut = createbranch.ErrBranchCheckedOut
)

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
	MergeBase(ctx context.Context, dir, a, b string) (string, error)
	CommitsSince(ctx context.Context, dir, base, tip string) ([]BranchCommit, error)
	// Config is what git uses for key in the clone: user.email is the actor's.
	Config(ctx context.Context, dir, key string) (string, error)
	// ChangeFingerprint identifies the change tip makes to base, apart from where
	// it sits, so two branches can be compared by what they do.
	ChangeFingerprint(ctx context.Context, dir, base, tip string) (string, error)
	// ForceBranch points a branch at a commit, creating or moving it.
	ForceBranch(ctx context.Context, dir, name, commit string) error
	// PushReplacing pushes over a branch that is at expected on the remote.
	PushReplacing(ctx context.Context, dir, remote, branch, expected string) error
	// PushCommitReplacing is PushReplacing for a commit, whatever the local branch.
	PushCommitReplacing(ctx context.Context, dir, remote, branch, commit, expected string) error
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

func (GitCLI) MergeBase(ctx context.Context, dir, a, b string) (string, error) {
	return gitcli.MergeBase(ctx, dir, a, b)
}

func (GitCLI) CommitsSince(ctx context.Context, dir, base, tip string) ([]BranchCommit, error) {
	facts, err := gitcli.CommitsSince(ctx, dir, base, tip)
	out := make([]BranchCommit, len(facts))
	for i, f := range facts {
		out[i] = BranchCommit{Hash: f.Hash, AuthorEmail: f.AuthorEmail, Message: f.Message}
	}
	return out, err
}

func (GitCLI) ChangeFingerprint(ctx context.Context, dir, base, tip string) (string, error) {
	return gitcli.ChangeFingerprint(ctx, dir, base, tip)
}

func (GitCLI) ForceBranch(ctx context.Context, dir, name, commit string) error {
	return gitcli.ForceBranch(ctx, dir, name, commit)
}

func (GitCLI) PushCommitReplacing(ctx context.Context, dir, remote, branch, commit, expected string) error {
	return gitcli.PushCommitReplacing(ctx, dir, remote, branch, commit, expected)
}

func (GitCLI) PushReplacing(ctx context.Context, dir, remote, branch, expected string) error {
	return gitcli.PushReplacing(ctx, dir, remote, branch, expected)
}

func (GitCLI) Config(ctx context.Context, dir, key string) (string, error) {
	return gitcli.ConfigIn(ctx, dir, key)
}

func (GitCLI) Push(ctx context.Context, dir, remote, branch string) error {
	return gitcli.Push(ctx, dir, remote, branch)
}

func (GitCLI) Discard(ctx context.Context, dir string, created []string) error {
	return gitcli.Discard(ctx, dir, created)
}

// PrepareRequest says which branch to prepare, and from what.
type PrepareRequest = createbranch.Input

// Prepare makes the new branch with the createbranch task, in place or in a
// worktree. It creates nothing when it fails.
func Prepare(ctx context.Context, g Git, req PrepareRequest) (*Work, error) {
	out, err := createbranch.Run(ctx, g, req)
	return out.Work, err
}

// short abbreviates a commit for a message.
func short(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}
