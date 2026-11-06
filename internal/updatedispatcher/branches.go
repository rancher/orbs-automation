package updatedispatcher

import (
	"errors"
	"fmt"

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
func (r Route) BranchesFor(producer config.Project, line string) ([]string, error) {
	consumer := r.Consumer
	if r.Edge.AcceptLines(producer) == config.AcceptLinesUniversal {
		branches := consumer.ReleaseBranches()
		if len(branches) == 0 {
			return nil, fmt.Errorf("%w: %s has no release branches", ErrNoBranches, consumer.Repo.Name)
		}
		return branches, nil
	}
	if line == "" {
		return nil, fmt.Errorf("%w: the release is in none of the release lines of %s, so there is no branch to follow it to",
			ErrNoBranches, producer.Repo.Name)
	}
	for _, l := range consumer.Release.Lines {
		if l.Line == line {
			branch, _ := consumer.Branch(l)
			return []string{branch}, nil
		}
	}
	return nil, fmt.Errorf("%w: %s has no release line %s, which the release is in", ErrNoBranches, consumer.Repo.Name, line)
}
