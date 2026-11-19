package backport

import (
	"context"

	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
)

// Git is what a backport needs from git: reading how a PR landed on its base, and the cherry-pick
// with its stops. Every operation names the directory it runs in, which is the work directory.
// It is defined here, where it is used, and GitCLI is the implementation.
type Git interface {
	Fetch(ctx context.Context, dir, remote, branch string) (commit string, err error)
	ParentCount(ctx context.Context, dir, commit string) (int, error)
	FirstParentLog(ctx context.Context, dir, tip string, n int) ([]gitcli.Logged, error)
	HoldsPickOf(ctx context.Context, dir, ref, commit string) (bool, error)
	// PatchPresent reports whether upstream has a commit with the same change as commit.
	PatchPresent(ctx context.Context, dir, upstream, commit string) (bool, error)
	// IsAncestor reports whether a is an ancestor of b.
	IsAncestor(ctx context.Context, dir, a, b string) (bool, error)
	CherryPick(ctx context.Context, dir string, p gitcli.Pick) (commit string, err error)
	Unmerged(ctx context.Context, dir string) ([]string, error)
	ConflictMarkers(dir string, files []string) ([]string, error)
	ContinuePick(ctx context.Context, dir string, files []string, p gitcli.Pick) (commit string, err error)
	AbortPick(ctx context.Context, dir string) error
}

// GitCLI is Git through the git binary (internal/platform/git/gitcli).
type GitCLI struct{}

var _ Git = GitCLI{}

func (GitCLI) Fetch(ctx context.Context, dir, remote, branch string) (string, error) {
	return gitcli.Fetch(ctx, dir, remote, branch)
}

func (GitCLI) ParentCount(ctx context.Context, dir, commit string) (int, error) {
	return gitcli.ParentCount(ctx, dir, commit)
}

func (GitCLI) FirstParentLog(ctx context.Context, dir, tip string, n int) ([]gitcli.Logged, error) {
	return gitcli.FirstParentLog(ctx, dir, tip, n)
}

func (GitCLI) HoldsPickOf(ctx context.Context, dir, ref, commit string) (bool, error) {
	return gitcli.HoldsPickOf(ctx, dir, ref, commit)
}

func (GitCLI) CherryPick(ctx context.Context, dir string, p gitcli.Pick) (string, error) {
	return gitcli.CherryPick(ctx, dir, p)
}

func (GitCLI) Unmerged(ctx context.Context, dir string) ([]string, error) {
	return gitcli.Unmerged(ctx, dir)
}

func (GitCLI) ConflictMarkers(dir string, files []string) ([]string, error) {
	return gitcli.ConflictMarkers(dir, files)
}

func (GitCLI) ContinuePick(ctx context.Context, dir string, files []string, p gitcli.Pick) (string, error) {
	return gitcli.ContinuePick(ctx, dir, files, p)
}

func (GitCLI) AbortPick(ctx context.Context, dir string) error { return gitcli.AbortPick(ctx, dir) }

func (GitCLI) PatchPresent(ctx context.Context, dir, upstream, commit string) (bool, error) {
	return gitcli.PatchPresent(ctx, dir, upstream, commit)
}

func (GitCLI) IsAncestor(ctx context.Context, dir, a, b string) (bool, error) {
	return gitcli.IsAncestor(ctx, dir, a, b)
}
