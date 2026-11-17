package ship

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
	"github.com/stretchr/testify/require"
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

const (
	me           = "danpock"
	trackerBlock = "<!-- gero:tracker v1\nissue: 12\nstate: a > b\n-->"
)

// pr is an open PR by author with the marker of a bump at a version.
func pr(number int, author string, m Marker) pulls.PR {
	return proposaltest.PR(number, author, m.String())
}

// mustPrepare prepares and requires a work directory that exists, so a test that
// edits in it cannot write into the package directory when Prepare does nothing.
func mustPrepare(t *testing.T, req PrepareRequest) *Work {
	t.Helper()
	w, err := Prepare(bg, GitCLI{}, req)
	require.NoError(t, err)
	require.NotNil(t, w)
	require.DirExists(t, w.Dir)
	return w
}

// The fakes of proposaltest, under the names the tests of this package use.
type (
	fakeRemote = proposaltest.FakeRemote
	log        = proposaltest.Log
)
