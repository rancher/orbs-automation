package ship

import "github.com/rancher/orbs-automation/internal/github/pulls"

// PublishAction is what to do about the PR of one target.
type PublishAction string

// The actions.
const (
	// PublishCreate: open the PR, then close the older ones of the actor's own.
	PublishCreate PublishAction = "create"
	// PublishAlreadyOpen: the actor's own PR at this version is open; make none.
	PublishAlreadyOpen PublishAction = "already open"
	// PublishBlocked: the actor's own PR at a newer version is open, so this
	// release is the older one and the target is left alone.
	PublishBlocked PublishAction = "blocked"
)

// Publish is the decision for one target.
type Publish struct {
	Action PublishAction
	// Open is the PR that is already open at this version, for PublishAlreadyOpen.
	Open *pulls.PR
	// Newer is the PR at a newer version, for PublishBlocked.
	Newer *pulls.PR
	// Supersede is the actor's own PRs to comment on and close, once the new one
	// exists (or, for PublishAlreadyOpen, because they are older than the open one).
	// It is empty when the target is blocked.
	Supersede []pulls.PR
	// Also is the PRs of others with this bump's key at this version or a newer
	// one. They are not touched, and are only reported.
	Also []pulls.PR
}

// decidePublish decides what to do for a bump, from the open PRs found for its
// base branch. Only PRs with the bump's key (source, consumer and target) count,
// and only those opened by me, the login that is running, are the actor's own to
// close; the others are never touched and do not block, though a same or newer
// one is reported.
//
// STUB: always create, touching nothing, until a later commit.
func decidePublish(open []pulls.PR, me string, m Marker) Publish {
	return Publish{Action: PublishCreate}
}
