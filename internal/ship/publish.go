package ship

import (
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

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
// base branch, newest first. Only PRs with the bump's key (source, consumer and
// target) count, and only those opened by me, the login that is running, are the
// actor's own to close; the others are never touched and do not block, though a
// same or newer one is reported. A version that cannot be ordered against this
// one is not acted on.
func decidePublish(open []pulls.PR, me string, m Marker) Publish {
	var out Publish
	var older []pulls.PR
	for _, p := range open {
		found, ok := ParseMarker(p.Body)
		if !ok || !found.SameBump(m) {
			continue
		}
		order, ordered := compareVersions(found.Version, m.Version)
		if p.Author != me {
			if ordered && order >= 0 || !ordered && found.Version == m.Version {
				out.Also = append(out.Also, p)
			}
			continue
		}
		switch {
		case !ordered && found.Version != m.Version:
			// Not ordered against this release, so not closed on a guess.
		case order > 0:
			if out.Newer == nil {
				newer := p
				out.Newer = &newer
			}
		case order == 0:
			if out.Open == nil {
				open := p
				out.Open = &open
			} else {
				older = append(older, p) // a duplicate of the one kept
			}
		default:
			older = append(older, p)
		}
	}
	switch {
	case out.Newer != nil:
		out.Action = PublishBlocked
	case out.Open != nil:
		out.Action, out.Supersede = PublishAlreadyOpen, older
	default:
		out.Action, out.Supersede = PublishCreate, older
	}
	return out
}

// compareVersions orders two tags by SemVer precedence: negative when a is older
// than b. ok is false when either is not a version.
func compareVersions(a, b string) (order int, ok bool) {
	va, okA := semver.Parse(a)
	vb, okB := semver.Parse(b)
	if !okA || !okB {
		return 0, false
	}
	return semver.Compare(va, vb), true
}
