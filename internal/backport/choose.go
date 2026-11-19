// Package backport carries a merged pull request down to the release branches of its repo,
// one pull request for each: which commits to pick for the way the PR landed, the targets, the
// picks with their stop on a conflict, and the texts and identity of each PR. It is a source of
// proposals for internal/proposal, as internal/ship and internal/reposync are; it decides what
// to propose and never how a PR is made (docs/specs/BACKPORT.md).
package backport

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

// Shape is how a PR landed on its base.
type Shape string

// The ways a PR lands.
const (
	// ShapeSingle is one commit, which a squash or a rebase of one commit also leaves.
	ShapeSingle Shape = "single"
	// ShapeRebase is a rebase merge: each commit of the PR, rewritten onto the base.
	ShapeRebase Shape = "rebase"
	// ShapeSquash is a squash merge: one commit for a PR of several.
	ShapeSquash Shape = "squash"
	// ShapeMerge is a merge commit.
	ShapeMerge Shape = "merge"
)

// PR is a merged pull request, as far as a backport needs it.
type PR struct {
	Repo        string // owner/name
	Number      int
	Title       string
	URL         string
	Base        string // the branch it was merged to
	MergeCommit string // what the host records as the result of the merge
	Commits     []pulls.Commit
}

// Picks is what to cherry-pick for a PR, in order, and how it was found.
type Picks struct {
	Shape   Shape
	Commits []string // oldest first
	// Mainline is the parent a merge commit is picked against, or 0.
	Mainline int
}

// Choose says what to pick for a PR from how it landed, as a person would read it off the base
// branch: the merge commit's parents, and whether the commits before it are the PR's own commits
// rewritten. It reads the base in dir, which has fetched it. What it cannot read is an error that
// says so, never a guess.
func Choose(ctx context.Context, g Git, dir string, pr PR) (Picks, error) {
	if pr.MergeCommit == "" {
		return Picks{}, fmt.Errorf("%s#%d records no merge commit, so there is nothing to read how it landed from", pr.Repo, pr.Number)
	}
	if len(pr.Commits) == 0 {
		return Picks{}, fmt.Errorf("%s#%d lists no commits, so there is nothing to tell a squash from a rebase by", pr.Repo, pr.Number)
	}
	parents, err := g.ParentCount(ctx, dir, pr.MergeCommit)
	if err != nil {
		return Picks{}, fmt.Errorf("read the merge commit %s of %s#%d: %w", pr.MergeCommit, pr.Repo, pr.Number, err)
	}
	switch {
	case parents > 1:
		return Picks{Shape: ShapeMerge, Commits: []string{pr.MergeCommit}, Mainline: 1}, nil
	case len(pr.Commits) == 1:
		return Picks{Shape: ShapeSingle, Commits: []string{pr.MergeCommit}}, nil
	}
	// Several commits behind one parent: a rebase merge puts each of them on the base again,
	// with the message and the author they had, ending at the merge commit; a squash puts one.
	tail, err := g.FirstParentLog(ctx, dir, pr.MergeCommit, len(pr.Commits))
	if err != nil {
		return Picks{}, fmt.Errorf("read the base before %s for %s#%d: %w", pr.MergeCommit, pr.Repo, pr.Number, err)
	}
	if rewritten(tail, pr.Commits) {
		hashes := make([]string, len(tail))
		for i, c := range tail {
			hashes[i] = c.Hash
		}
		return Picks{Shape: ShapeRebase, Commits: hashes}, nil
	}
	return Picks{Shape: ShapeSquash, Commits: []string{pr.MergeCommit}}, nil
}

// rewritten reports whether the commits on the base are the PR's own, in order, rewritten:
// the same number, and each with the message and the author of its original.
func rewritten(onBase []gitcli.Logged, own []pulls.Commit) bool {
	if len(onBase) != len(own) {
		return false
	}
	for i := range own {
		if strings.TrimSpace(onBase[i].Message) != strings.TrimSpace(own[i].Message) || onBase[i].Author != own[i].Author {
			return false
		}
	}
	return true
}
