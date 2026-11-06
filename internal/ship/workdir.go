package ship

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
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
type PrepareRequest struct {
	CloneDir string // the engineer's clone of the consumer
	Remote   string // the remote the target branch is fetched from
	Target   string // the consumer branch the bump goes to, such as release/v2.15
	Base     string // the commit of Target the state check read
	Branch   string // the new branch to create
	DryRun   bool   // never move the checkout
	TempDir  string // where a worktree goes
	// Rebuild builds the branch again even when one of ours is there and current.
	Rebuild bool
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
		reused, err := settleExisting(ctx, g, req, commit)
		if err != nil {
			return nil, err
		}
		if reused != nil {
			return reused, nil
		}
	}
	collisions, err := g.Collisions(ctx, req.CloneDir, req.Base, st.Untracked)
	if err != nil {
		return nil, err
	}
	choice := workdir.ChoosePlace(workdir.CloneState{Branch: st.Branch, InProgress: st.InProgress, Tracked: st.Tracked, Collisions: collisions}, req.DryRun)
	var w *Work
	switch choice.Place {
	case InPlace:
		if err := g.StartBranch(ctx, req.CloneDir, req.Branch, req.Base); err != nil {
			return nil, err
		}
		w = workdir.NewInPlace(g, req.CloneDir, req.Branch, st.Branch, st.Untracked)
	default:
		dir := filepath.Join(req.TempDir, strings.ReplaceAll(req.Branch, "/", "-"))
		if err := g.AddWorktree(ctx, req.CloneDir, dir, req.Branch, req.Base); err != nil {
			return nil, err
		}
		w = workdir.NewInWorktree(g, req.CloneDir, dir, req.Branch, choice.Reason)
	}
	return w, nil
}

// settleExisting decides what to do about a branch of the name that is already
// there, at commit. One that is at the base itself holds nothing, and goes. One
// whose commits are all ours (see git.CommitsAreOurs) is the actor's own earlier work:
// when it is based on the current head of the target it is reused as it is, which
// is also how a run whose push failed is resumed, and otherwise, or when a rebuild
// was asked for, it goes and is built again. One with a commit that is not ours is
// an error, since those commits are somebody's to keep. A nil Work with no error
// says to go on and build.
func settleExisting(ctx context.Context, g Git, req PrepareRequest, commit string) (*Work, error) {
	if commit != req.Base {
		mergeBase, err := g.MergeBase(ctx, req.CloneDir, req.Base, commit)
		if err != nil {
			return nil, fmt.Errorf("compare the branch %s with %s: %w", req.Branch, req.Target, err)
		}
		facts, err := g.CommitsSince(ctx, req.CloneDir, mergeBase, commit)
		if err != nil {
			return nil, err
		}
		email, err := g.Config(ctx, req.CloneDir, "user.email")
		if err != nil {
			return nil, err
		}
		if ours, why := git.CommitsAreOurs(facts, email); !ours {
			return nil, fmt.Errorf("%w: %s is at %s, and %s: delete it or keep it, it is not ours to change", ErrBranchExists, req.Branch, short(commit), why)
		}
		if mergeBase == req.Base && !req.Rebuild {
			existing := make([]CommitInfo, len(facts))
			for i, f := range facts {
				subject, _, _ := strings.Cut(f.Message, "\n")
				existing[i] = CommitInfo{Hash: f.Hash, Subject: subject}
			}
			return workdir.NewExisting(g, req.CloneDir, req.Branch, existing), nil
		}
	}
	// At the base, or ours and out of date, or to be rebuilt: nothing is lost.
	if err := g.DeleteBranch(ctx, req.CloneDir, req.Branch); err != nil {
		return nil, err
	}
	return nil, nil
}

// short abbreviates a commit for a message.
func short(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}
