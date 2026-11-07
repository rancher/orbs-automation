package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ConsumerHoldings is what one consumer of a project holds of its releases, branch
// by branch: the read half of a bump, with nothing written and nothing proposed.
// It is what gero repo status shows of who has picked a release up.
type ConsumerHoldings struct {
	Consumer string `json:"consumer"` // the repository, owner/name
	Name     string `json:"name"`     // its config name
	Method   string `json:"method"`   // the update method the project ships with
	// Error is why this consumer could not be traced at all: its branches could not
	// be read, or no branch or release could be worked out. The other consumers are
	// traced all the same.
	Error   string    `json:"error,omitempty"`
	Targets []Holding `json:"targets,omitempty"`
}

// Holding is one consumer branch that releases of the project go to.
type Holding struct {
	// Line is the Rancher line of the consumer's branch, "" when it is in none.
	Line   string `json:"line,omitempty"`
	Branch string `json:"branch"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	// Current is what the branch holds now, as the strategy reads it.
	Current string `json:"current,omitempty"`
	// Stable is how that stands against the latest stable release that goes to the
	// branch, and Newest against the newest release of any kind, which is the same
	// release when no prerelease is newer.
	Stable *Verdict `json:"stable,omitempty"`
	Newest *Verdict `json:"newest,omitempty"`
	// Error is why the branch could not be read: it is not on the host, or the
	// strategy found no value in its file.
	Error string `json:"error,omitempty"`
}

// Verdict is how what a branch holds stands against one release.
type Verdict struct {
	Tag      string                    `json:"tag"`  // the release
	Want     string                    `json:"want"` // what the branch would hold for it
	UpToDate bool                      `json:"upToDate"`
	Relation updatedispatcher.Relation `json:"relation"` // the release against what is held: newer means the branch is behind
}

// Holdings traces every consumer of producer: for each route out of it, the
// consumer's branches that its releases go to and what each holds, against the
// latest stable release and the newest of any kind. It uses the pieces a run of
// ship does, so the branches and the release of each are the ones ship would pick,
// and a consumer's file is read once for each branch.
//
// STUB: traces nothing until a later commit.
func Holdings(ctx context.Context, ws Reader, producer config.Project) ([]ConsumerHoldings, error) {
	return nil, nil
}
