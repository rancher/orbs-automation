package proposal

import (
	"context"

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
