package ship

import (
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holdings traces the producer of the workspace, which the fixtures name as
// ship would be asked for it.
func holdings(t *testing.T, ws *fakeWorkspace) ([]ConsumerHoldings, error) {
	t.Helper()
	producer, err := ws.Project("")
	require.NoError(t, err)
	return Holdings(ctx, ws, producer)
}

func byBranch(t *testing.T, c ConsumerHoldings) map[string]Holding {
	t.Helper()
	out := map[string]Holding{}
	for _, h := range c.Targets {
		out[h.Branch] = h
	}
	return out
}

func reads(ws *fakeWorkspace, prefix string) int {
	n := 0
	for _, a := range ws.asked {
		if strings.HasPrefix(a, prefix) {
			n++
		}
	}
	return n
}

func TestHoldingsOfAUniversalProducerAreEveryConsumerBranchAgainstTheLatestStableAndNewest(t *testing.T) {
	ws := withDevLine(loopFixture())

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	c := got[0]
	assert.Equal(t, "rancher/rancher", c.Consumer)
	assert.Equal(t, "build-yaml-bump", c.Method)
	assert.Empty(t, c.Error)
	hs := byBranch(t, c)
	require.Len(t, hs, 3, "every release branch of the consumer")

	behind := hs["release/v2.15"]
	assert.Equal(t, "v2.15", behind.Line)
	assert.Equal(t, "sha-215", behind.Head)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", behind.Current)
	require.NotNil(t, behind.Stable)
	assert.Equal(t, Verdict{Tag: "v0.5.3", Want: "rancher/scc-operator:v0.5.3", UpToDate: false, Relation: updatedispatcher.RelationNewer}, *behind.Stable)
	require.NotNil(t, behind.Newest)
	assert.Equal(t, "v0.6.0-rc.1", behind.Newest.Tag, "the newest release of any kind")
	assert.Equal(t, updatedispatcher.RelationNewer, behind.Newest.Relation)

	current := hs["main"]
	assert.Equal(t, "v2.16", current.Line)
	require.NotNil(t, current.Stable)
	assert.True(t, current.Stable.UpToDate, "main holds the latest stable")
	assert.Equal(t, updatedispatcher.RelationSame, current.Stable.Relation)
	assert.Equal(t, "v0.6.0-rc.1", current.Newest.Tag)
	assert.False(t, current.Newest.UpToDate, "an rc is newer than what it holds")

	assert.Equal(t, "rancher/scc-operator:v0.5.1", hs["release/v2.14"].Current)
}

func TestHoldingsWithNoNewerPrereleaseHaveTheSameVerdictForBoth(t *testing.T) {
	ws := loopFixture()
	ws.refs["rancher/scc-operator"] = git.Refs{Tags: tags("v0.5.2", "v0.5.3")}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	for _, h := range got[0].Targets {
		require.NotNil(t, h.Stable, h.Branch)
		require.NotNil(t, h.Newest, h.Branch)
		assert.Equal(t, *h.Stable, *h.Newest, h.Branch)
	}
}

func TestHoldingsReadEachFileOnceAndEachRepositoryOnce(t *testing.T) {
	ws := withDevLine(loopFixture())

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 3, reads(ws, "file "), "one read for each branch, though it is compared twice")
	assert.Equal(t, 1, reads(ws, "refs rancher/rancher"))
	assert.Equal(t, 1, reads(ws, "refs rancher/scc-operator"))
}

func TestHoldingsOfAProducerWithLinesGiveEachBranchTheLatestOfItsOwnSeries(t *testing.T) {
	ws := laned(t, "v8.0.9", "v8.1.0", "v9.0.0", "v9.0.1-rc.1")

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	hs := byBranch(t, got[0])
	require.Len(t, hs, 2, "the lines both have")

	main := hs["main"]
	require.NotNil(t, main.Stable)
	assert.Equal(t, "v9.0.0", main.Stable.Tag)
	assert.Equal(t, "v9.0.1-rc.1", main.Newest.Tag)
	assert.Equal(t, "rancher/kuberlr:v8.9.0", main.Current)

	v215 := hs["release/v2.15"]
	assert.Equal(t, "v8.1.0", v215.Stable.Tag)
	assert.Equal(t, "v8.1.0", v215.Newest.Tag, "no prerelease of its series is newer")
}

func TestHoldingsABranchWithNoValueIsAnErrorOfItsOwn(t *testing.T) {
	ws := loopFixture()
	delete(ws.files, "rancher/rancher@sha-214:build.yaml")

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	hs := byBranch(t, got[0])
	assert.Contains(t, hs["release/v2.14"].Error, "build.yaml")
	assert.Nil(t, hs["release/v2.14"].Stable)
	assert.Empty(t, hs["release/v2.15"].Error, "the others are traced")
	assert.NotNil(t, hs["release/v2.15"].Stable)
}

func TestHoldingsAConsumerThatCannotBeReadIsReportedAndDoesNotStopTheRest(t *testing.T) {
	ws := loopFixture()
	ws.refsErr = map[string]error{"rancher/rancher": errors.New("network down")}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "rancher/rancher", got[0].Consumer)
	assert.Contains(t, got[0].Error, "network down")
	assert.Empty(t, got[0].Targets)
}

func TestHoldingsAProducerWithNoReleaseHasAConsumerErrorAndNoTargets(t *testing.T) {
	ws := loopFixture()
	ws.refs["rancher/scc-operator"] = git.Refs{}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Error, "no release")
}

func TestHoldingsOfAProducerThatShipsNowhereAreNone(t *testing.T) {
	ws := loopFixture()
	ws.links = nil

	got, err := holdings(t, ws)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSourcesAreWhatTheConsumerTakesFromEachProjectThatShipsToIt(t *testing.T) {
	ws := withDevLine(loopFixture())
	consumer := ws.links[0].Consumer

	got, err := Sources(ctx, ws, consumer)

	require.NoError(t, err)
	require.Len(t, got, 1)
	src := got[0]
	assert.Equal(t, "rancher/scc-operator", src.Producer)
	assert.Equal(t, "build-yaml-bump", src.Method)
	assert.Empty(t, src.Error)
	require.Len(t, src.Targets, 3, "the consumer's own branches")
	var behind Holding
	for _, h := range src.Targets {
		if h.Branch == "release/v2.15" {
			behind = h
		}
	}
	assert.Equal(t, "rancher/scc-operator:v0.5.2", behind.Current)
	require.NotNil(t, behind.Stable)
	assert.Equal(t, "v0.5.3", behind.Stable.Tag)
	assert.Equal(t, updatedispatcher.RelationNewer, behind.Stable.Relation)
}

func TestSourcesOfAProjectNothingShipsToAreNone(t *testing.T) {
	ws := loopFixture()

	got, err := Sources(ctx, ws, ws.producer)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSourcesASourceThatCannotBeTracedIsReportedWithItsError(t *testing.T) {
	ws := loopFixture()
	ws.refsErr = map[string]error{"rancher/scc-operator": errors.New("network down")}

	got, err := Sources(ctx, ws, ws.links[0].Consumer)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "rancher/scc-operator", got[0].Producer)
	assert.Contains(t, got[0].Error, "network down")
}
