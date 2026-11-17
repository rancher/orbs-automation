package reposync

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
)

var bg = context.Background()

// The helpers of proposaltest, under the short names the tests of this package use.
var (
	runGit   = proposaltest.RunGit
	put      = proposaltest.Put
	slurp    = proposaltest.Slurp
	remote   = proposaltest.Remote
	engineer = proposaltest.Engineer
	current  = proposaltest.Current
)

type (
	fakeRemote = proposaltest.FakeRemote
	log        = proposaltest.Log
)

// pr is an open PR by author with the marker of a proposal.
func pr(number int, author string, m proposal.Marker) pulls.PR {
	return proposaltest.PR(number, author, m.String())
}

func branchExistsIn(t *testing.T, dir, name string) bool {
	t.Helper()
	return runGit(t, dir, "branch", "--list", name) != ""
}
