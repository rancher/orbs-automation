package ship

// BranchKey is the part of a bump's branch name that gero builds and the user
// cannot change: the producer's project name, the release tag and the target
// branch without release/, joined by hyphens. It always ends the name, so the
// branches of one bump share a tail whatever prefix the actor uses
// (docs/specs/PUSH_TO_REPO.md#branches-and-pr-identity).
func BranchKey(source, tag, target string) string {
	return ""
}
