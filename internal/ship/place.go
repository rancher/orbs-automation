package ship

// Place is where the branch for one target is prepared.
type Place string

const (
	// InPlace is the engineer's own checkout, switched to the new branch and put
	// back afterwards.
	InPlace Place = "in place"
	// Worktree is a temporary worktree of the same repository, which leaves the
	// engineer's checkout alone.
	Worktree Place = "worktree"
)

// CloneState is what the choice of Place needs to know about the engineer's
// clone for one target branch.
type CloneState struct {
	Branch     string   // the checked-out branch, "" when HEAD is detached
	InProgress string   // an unfinished merge, rebase, cherry-pick, revert or bisect, "" for none
	Tracked    []string // tracked files that are staged, modified or deleted
	Collisions []string // untracked files the target branch's tree would overwrite
}

// Choice is where to work and why not in place, for the report.
type Choice struct {
	Place  Place
	Reason string // empty when the choice is InPlace
}

// ChoosePlace decides where to prepare the branch for one target. It is in
// place only when the clone is on a branch, with no operation in progress, no
// change to a tracked file and no untracked file in the way, and this is not a
// dry run. Otherwise it is a worktree, and the reason says which.
//
// STUB: always a worktree, with no reason, until a later commit.
func ChoosePlace(s CloneState, dryRun bool) Choice {
	return Choice{Place: Worktree}
}
