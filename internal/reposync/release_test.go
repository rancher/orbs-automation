package reposync

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	extHeld = "prodVersion: v9 # keep this\nextVersion: v1.35.9+k3s1 # the k3s under test\nother: 1\n"
	extNow  = "prodVersion: v8 # keep this\nextVersion: v1.35.10+k3s1 # the k3s under test\nother: 1\n"
)

func tagsOf(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

// releaseDocs is a producer that nobody ships, whose releases of one minor the holder
// holds in build.yaml: the newest patch of v1.35, as PromFed holds k3s.
func releaseDocs(followed ...string) []string {
	docs := append(seriesDocs(false),
		chaintest.Doc("Project", "ext", "", "  repo: {name: acme/ext, external: true}\n"),
		chaintest.Doc("Stream", "ext-1-35", "{project: ext}", "  scheme: semver\n  tagPattern: \"v1.35.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "ext-1-35-img", "{stream: ext-1-35}", "  kind: image\n  name: acme/ext\n"),
		chaintest.Doc("Holding", "cons-holds-ext", "{project: cons, artifact: ext-1-35-img}", "  at: {file: build.yaml, key: extVersion}\n  holds: {kind: release, form: \"{tag}\"}\n"),
	)
	if len(followed) == 0 {
		followed = []string{"cons-holds-prod", "cons-holds-side", "cons-holds-ext"}
	}
	return append(docs, chaintest.Doc("Edit", "regen", "{holding: ["+strings.Join(followed, ", ")+"]}", "  postEdit:\n    - {run: ./scripts/gen.sh}\n"))
}

// newReleaseWS is the holder with the producer's tags on the host.
func newReleaseWS(t *testing.T, held string, tags []git.Tag, followed ...string) (*seriesWS, *proposal.Executor) {
	t.Helper()
	up := seriesRepoWith(t, held)
	ws := &seriesWS{
		t: t, graph: chaintest.Load(t, releaseDocs(followed...)...), up: up, clone: engineer(t, up),
		tags: map[string][]git.Tag{"acme/ext": tags},
	}
	return ws, &proposal.Executor{Git: proposal.GitCLI{}, Runner: steprun.Runner{}, TempDir: t.TempDir()}
}

func rowOf(t *testing.T, b Branch) Row {
	t.Helper()
	for _, r := range b.Rows {
		if r.Holding == "cons-holds-ext" {
			return r
		}
	}
	require.Failf(t, "no row", "the release holding in %+v", b.Rows)
	return Row{}
}

var extTags = tagsOf("v1.35.8+k3s1", "v1.35.9+k3s1", "v1.35.10-rc1+k3s1", "v1.35.10+k3s1", "v1.36.0+k3s1", "main-abc")

func TestSyncPullsTheNewestStablePatchOfAReleaseHolding(t *testing.T) {
	ws, _ := newReleaseWS(t, extHeld, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusReady, row.Status)
	assert.Equal(t, "v1.35.9+k3s1", row.Current)
	assert.Equal(t, "v1.35.10+k3s1", row.Want, "10 is newer than 9, and a release candidate is never taken")
	assert.Equal(t, "main", row.Branch)
}

func TestSyncWritesAReleaseHoldingAndASeriesHoldingInOneCommit(t *testing.T) {
	ws, ex := newReleaseWS(t, extHeld, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true}, ex)

	require.NoError(t, err)
	b := branchOf(t, plan, behind)
	require.NotNil(t, b.Result)
	assert.Equal(t, proposal.ResultCommitted, b.Result.Status)
	assert.Equal(t, extNow, runGit(t, ws.clone, "show", b.Result.Branch+":build.yaml")+"\n", "the series and the release are both written, comments kept")
	assert.Equal(t, "main", current(t, ws.clone))
	assert.Equal(t, extHeld, slurp(t, filepath.Join(ws.clone, "build.yaml")), "the engineer's checkout is untouched")
}

func TestSyncTakesTheNewestRespinOfAPatch(t *testing.T) {
	ws, _ := newReleaseWS(t, extHeld, tagsOf("v1.35.9+k3s1", "v1.35.10+k3s1", "v1.35.10+k3s2", "v1.35.10-rc2+k3s3"))

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	assert.Equal(t, "v1.35.10+k3s2", rowOf(t, branchOf(t, plan, "main")).Want)
}

func TestSyncRewritesTheSpellingOfARelease(t *testing.T) {
	held := "prodVersion: v9\nextVersion: v1.35.10-k3s1\nother: 1\n" // the OCI spelling, the same release
	ws, _ := newReleaseWS(t, held, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusReady, row.Status, "the file says the same release in another spelling, and the form of the holding is the one written")
	assert.Equal(t, "v1.35.10+k3s1", row.Want)
}

func TestSyncMovesAReleaseHoldingBackwardsWhenItIsWrong(t *testing.T) {
	held := "prodVersion: v9\nextVersion: v1.35.12+k3s1\nother: 1\n" // newer than any stable release there is
	ws, _ := newReleaseWS(t, held, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusReady, row.Status, "ship never moves a holding backwards; sync is what corrects one that is wrong")
	assert.Equal(t, "v1.35.10+k3s1", row.Want)
}

func TestSyncAReleaseHoldingWithNoStableReleaseIsReportedNotWritten(t *testing.T) {
	ws, _ := newReleaseWS(t, extHeld, tagsOf("v1.35.10-rc1+k3s1"))

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusFailed, row.Status)
	assert.Contains(t, row.Reason, "no stable release")
}

func TestSyncDoesNotWriteAReleaseHoldingNoEditFollows(t *testing.T) {
	ws, ex := newReleaseWS(t, extHeld, extTags, "cons-holds-prod", "cons-holds-side")

	plan, err := Run(bg, ws, Input{NoPush: true}, ex)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusBlocked, row.Status)
	assert.Contains(t, row.Reason, "no Edit follows")
}

func TestSyncCanBeLimitedToNamedHoldings(t *testing.T) {
	ws, _ := newReleaseWS(t, extHeld, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true, Holdings: []string{"cons-holds-ext", "cons-holds-side"}}, nil)

	require.NoError(t, err)
	var names []string
	for _, b := range plan.Branches {
		for _, r := range b.Rows {
			names = append(names, r.Holding)
		}
	}
	assert.NotContains(t, names, "cons-holds-prod")
	assert.Contains(t, names, "cons-holds-ext")
	assert.Contains(t, names, "cons-holds-side")

	_, err = Run(bg, ws, Input{NoPush: true, Holdings: []string{"cons-holds-ext", "nope"}}, nil)
	require.ErrorContains(t, err, "nope")
}

func TestSyncCorrectsAReleaseHoldingThatIsInAnotherSeriesEvenIfItIsNewer(t *testing.T) {
	held := "prodVersion: v9\nextVersion: v1.36.2+k3s1\nother: 1\n" // the minor above the one this stream follows
	ws, _ := newReleaseWS(t, held, extTags)

	plan, err := Run(bg, ws, Input{NoPush: true}, nil)

	require.NoError(t, err)
	row := rowOf(t, branchOf(t, plan, "main"))
	assert.Equal(t, StatusReady, row.Status, "a holding on the wrong minor is corrected, not held back for being newer")
	assert.Equal(t, "v1.35.10+k3s1", row.Want)
}
