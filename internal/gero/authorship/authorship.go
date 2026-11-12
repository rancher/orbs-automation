package authorship

import (
	"fmt"
	"strings"
)

// Commit is a commit on a branch, as far as telling whose it is.
type Commit struct {
	Hash        string
	AuthorEmail string
	Message     string
}

// Trailer is the line every commit gero ship makes ends with.
const Trailer = "Automation: gero ship"

// AreOurs reports whether every commit was made by gero for this actor: it
// has the trailer on a line of its own, and its author is the actor's email,
// compared without regard to case. A branch of such commits is the actor's own
// work and can be rebuilt; one with a commit that is not is somebody's to keep.
// When it is not, why names the first commit that is not, and what is wrong. No
// commits is trivially ours, and with no email for the actor nothing is claimed.
func AreOurs(commits []Commit, actorEmail string) (ours bool, why string) {
	if len(commits) > 0 && actorEmail == "" {
		return false, "there is no author email for the actor to compare the commits with"
	}
	for _, c := range commits {
		short := shortHash(c.Hash)
		if !strings.EqualFold(c.AuthorEmail, actorEmail) {
			return false, fmt.Sprintf("commit %s is by %s, not %s", short, c.AuthorEmail, actorEmail)
		}
		if !hasTrailer(c.Message) {
			return false, fmt.Sprintf("commit %s has no %q trailer", short, Trailer)
		}
	}
	return true, ""
}

// hasTrailer reports whether a line of the message is the trailer.
func hasTrailer(message string) bool {
	for _, line := range strings.Split(message, "\n") {
		if strings.TrimSpace(line) == Trailer {
			return true
		}
	}
	return false
}

// shortHash abbreviates a commit to 8 characters for a message.
func shortHash(commit string) string {
	return commit[:min(len(commit), 8)]
}
