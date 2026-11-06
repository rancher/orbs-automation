package updatedispatcher

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/config"
)

// ErrNoBranches is returned when no consumer branch can be worked out for a
// release: the consumer has none to follow it to.
var ErrNoBranches = errors.New("no consumer branch for the release")

// BranchesFor returns the consumer's branches a release of the producer goes to,
// in config order, newest first. line is the release line the release is in, or
// "" when it is in none. The edge's lines decide: a universal edge goes to every
// release branch of the consumer, and one that follows lines goes to the
// consumer's branch for line.
//
// STUB: returns nothing until a later commit.
func (r Route) BranchesFor(producer config.Project, line string) ([]string, error) {
	return nil, nil
}
