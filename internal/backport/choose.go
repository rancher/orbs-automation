// Package backport carries a merged pull request down to the release branches of its repo,
// one pull request for each: which commits to pick for the way the PR landed, the targets, the
// picks with their stop on a conflict, and the texts and identity of each PR. It is a source of
// proposals for internal/proposal, as internal/ship and internal/reposync are; it decides what
// to propose and never how a PR is made (docs/specs/BACKPORT.md).
package backport

import (
	"context"

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
	return Picks{}, nil
}
