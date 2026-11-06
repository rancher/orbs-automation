package ship

// BranchCommit is a commit on a branch, as far as telling whose it is.
type BranchCommit struct {
	Hash        string
	AuthorEmail string
	Message     string
}

// ourTrailer is the line every commit gero ship makes ends with.
const ourTrailer = "Automation: gero ship"

// commitsAreOurs reports whether every commit was made by gero for this actor: it
// has the trailer on a line of its own, and its author is the actor's email,
// compared without regard to case. A branch of such commits is the actor's own
// work and can be rebuilt; one with a commit that is not is somebody's to keep.
// When it is not, why names the first commit that is not, and what is wrong. No
// commits is trivially ours.
//
// STUB: always ours until a later commit.
func commitsAreOurs(commits []BranchCommit, actorEmail string) (ours bool, why string) {
	return true, ""
}
