package ship

import (
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git"
)

// BranchKey is the part of a bump's branch name that gero builds and the user
// cannot change: the producer's project name, the release tag and the target
// branch without release/, joined by hyphens. It always ends the name, so the
// branches of one bump share a tail whatever prefix the actor uses
// (docs/specs/PUSH_TO_REPO.md#branches-and-pr-identity).
func BranchKey(source, tag, target string) string {
	return source + "-" + BranchTail(tag, target)
}

// BranchTail is what ends a bump's branch name whatever the project: the tag and
// the target without release/. A marker holds both, so a PR's head can be checked
// against its marker without knowing the producer's project name.
func BranchTail(tag, target string) string {
	return tag + "-" + strings.TrimPrefix(target, "release/")
}

// maxBumpBranches is how many branches of a bump a run asks GitHub for the PR of.
// Branches of a merged bump are often left, so a limit keeps a run to a few
// requests, and a run that found more says so.
const maxBumpBranches = 30

// sourceOfKey is the producer's project name in a bump's key, which is what the
// key says before the tail of the release and the target.
func sourceOfKey(key, tag, target string) string {
	return ""
}

// bumpBranches are the branches of a repository that are a branch of this bump,
// at any version and by anyone: a branch that ends in the target's label, with
// the producer's name in front of the tag, whatever the prefix. It only reads the
// names. A branch it finds is a hint: the PR of it is what its marker confirms.
func bumpBranches(branches []git.Branch, source, target string) []string {
	return nil
}
