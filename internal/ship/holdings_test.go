package ship

import (
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"

	"github.com/rancher/orbs-automation/internal/platform/git"
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

// view is a consumer branch with what its one holding holds, as the tests read it.
type view struct {
	Line, Branch, Head, Current, Stage, Error string
	Stable, Newest                            *Verdict
}

func byBranch(t *testing.T, c ConsumerHoldings) map[string]view {
	t.Helper()
	out := map[string]view{}
	for _, h := range c.Targets {
		v := view{Line: h.Line, Branch: h.Branch, Head: h.Head, Error: h.Error}
		if len(h.Held) > 0 {
			v.Current, v.Stage, v.Stable, v.Newest = h.Held[0].Current, h.Held[0].Stage, h.Held[0].Stable, h.Held[0].Newest
			if v.Error == "" {
				v.Error = h.Held[0].Error
			}
		}
		out[h.Branch] = v
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
	ws := withDevLine(loopFixture(t))

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	c := got[0]
	assert.Equal(t, "rancher/rancher", c.Consumer)
	assert.Empty(t, c.Error)
	hs := byBranch(t, c)
	require.Len(t, hs, 3, "every release branch of the consumer")

	behind := hs["release/v2.15"]
	assert.Equal(t, "v2.15", behind.Line)
	assert.Equal(t, "sha-215", behind.Head)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", behind.Current)
	require.NotNil(t, behind.Stable)
	assert.Equal(t, Verdict{Tag: "v0.5.3", Want: "rancher/scc-operator:v0.5.3", UpToDate: false, Relation: bump.RelationNewer}, *behind.Stable)
	require.NotNil(t, behind.Newest)
	assert.Equal(t, "v0.6.0-rc.1", behind.Newest.Tag, "the newest release of any kind")
	assert.Equal(t, bump.RelationNewer, behind.Newest.Relation)

	current := hs["main"]
	assert.Equal(t, "v2.16", current.Line)
	require.NotNil(t, current.Stable)
	assert.True(t, current.Stable.UpToDate, "main holds the latest stable")
	assert.Equal(t, bump.RelationSame, current.Stable.Relation)
	assert.Equal(t, "v0.6.0-rc.1", current.Newest.Tag)
	assert.False(t, current.Newest.UpToDate, "an rc is newer than what it holds")

	assert.Equal(t, "rancher/scc-operator:v0.5.1", hs["release/v2.14"].Current)
}

func TestHoldingsWithNoNewerPrereleaseHaveTheSameVerdictForBoth(t *testing.T) {
	ws := loopFixture(t)
	ws.refs["rancher/scc-operator"] = git.Refs{Tags: tags("v0.5.2", "v0.5.3")}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	for branch, h := range byBranch(t, got[0]) {
		require.NotNil(t, h.Stable, branch)
		require.NotNil(t, h.Newest, branch)
		assert.Equal(t, *h.Stable, *h.Newest, branch)
	}
}

func TestHoldingsReadEachFileOnceAndEachRepositoryOnce(t *testing.T) {
	ws := withDevLine(loopFixture(t))

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
	ws := loopFixture(t)
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
	ws := loopFixture(t)
	ws.refsErr = map[string]error{"rancher/rancher": errors.New("network down")}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "rancher/rancher", got[0].Consumer)
	assert.Contains(t, got[0].Error, "network down")
	assert.Empty(t, got[0].Targets)
}

func TestHoldingsAProducerWithNoReleaseHasAConsumerErrorAndNoTargets(t *testing.T) {
	ws := loopFixture(t)
	ws.refs["rancher/scc-operator"] = git.Refs{}

	got, err := holdings(t, ws)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Error, "no release")
}

func TestHoldingsOfAProducerThatShipsNowhereAreNone(t *testing.T) {
	ws := loopFixture(t)
	ws.links = nil

	got, err := holdings(t, ws)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSourcesAreWhatTheConsumerTakesFromEachProjectThatShipsToIt(t *testing.T) {
	ws := withDevLine(loopFixture(t))
	consumer := ws.config().Project("rancher")

	got, err := Sources(ctx, ws, consumer)

	require.NoError(t, err)
	require.Len(t, got, 1)
	src := got[0]
	assert.Equal(t, "rancher/scc-operator", src.Producer)
	assert.Empty(t, src.Error)
	require.Len(t, src.Targets, 3, "the consumer's own branches")
	behind := byBranch(t, ConsumerHoldings{Targets: src.Targets})["release/v2.15"]
	assert.Equal(t, "rancher/scc-operator:v0.5.2", behind.Current)
	require.NotNil(t, behind.Stable)
	assert.Equal(t, "v0.5.3", behind.Stable.Tag)
	assert.Equal(t, bump.RelationNewer, behind.Stable.Relation)
}

func TestSourcesOfAProjectNothingShipsToAreNone(t *testing.T) {
	ws := loopFixture(t)

	got, err := Sources(ctx, ws, ws.config().Project("scc-operator"))

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSourcesASourceThatCannotBeTracedIsReportedWithItsError(t *testing.T) {
	ws := loopFixture(t)
	ws.refsErr = map[string]error{"rancher/scc-operator": errors.New("network down")}

	got, err := Sources(ctx, ws, ws.config().Project("rancher"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "rancher/scc-operator", got[0].Producer)
	assert.Contains(t, got[0].Error, "network down")
}

const twoLocations = "  at:\n    - {file: build.yaml, key: defaultSccOperatorImage}\n    - {file: nested/build.yaml, key: image}\n"

// nestedFiles puts a nested file on each branch of loopFixture, holding the given
// image tags in the order main, release/v2.15, release/v2.14.
func nestedFiles(ws *fakeWorkspace, tags ...string) {
	for i, ref := range []string{"sha-main", "sha-215", "sha-214"} {
		ws.files["rancher/rancher@"+ref+":nested/build.yaml"] = "image: rancher/scc-operator:" + tags[i] + "\n"
	}
}

func TestHoldingsOfSeveralLocationsAgreeingHaveEachOfThem(t *testing.T) {
	ws := withDevLine(loopFixture(t))
	ws.links[0].Locations = twoLocations
	nestedFiles(ws, "v0.5.3", "v0.5.2", "v0.5.1") // each as the root file holds it

	got, err := holdings(t, ws)

	require.NoError(t, err)
	held := got[0].Targets[0].Held[0]
	require.Len(t, held.Locations, 2)
	assert.Equal(t, "build.yaml", held.Locations[0].File)
	assert.Equal(t, "nested/build.yaml", held.Locations[1].File)
	assert.False(t, held.Differ)
	assert.Empty(t, held.DifferReason)
}

func TestHoldingsOfSeveralLocationsThatDifferSayWhatEachHolds(t *testing.T) {
	ws := withDevLine(loopFixture(t))
	ws.links[0].Locations = twoLocations
	nestedFiles(ws, "v0.5.2", "v0.5.2", "v0.5.1") // main: the root holds v0.5.3, the nested file v0.5.2

	got, err := holdings(t, ws)

	require.NoError(t, err)
	var main HeldValue
	for _, h := range got[0].Targets {
		if h.Branch == "main" {
			main = h.Held[0]
		}
	}
	assert.True(t, main.Differ)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", main.Current, "the first location's value")
	assert.Equal(t, []HeldLocation{
		{File: "build.yaml", Current: "rancher/scc-operator:v0.5.3"},
		{File: "nested/build.yaml", Current: "rancher/scc-operator:v0.5.2"},
	}, main.Locations)
	assert.Empty(t, main.DifferReason, "nothing acknowledges it")
}

func TestHoldingsAnAcknowledgementOfTheDifferenceGivesItsReasonForTheLinesItNames(t *testing.T) {
	ws := withDevLine(loopFixture(t))
	ws.links[0].Locations = twoLocations
	ws.links[0].Acknowledge = "  - {rule: locations-differ, reason: nested modules lag on main, lines: {since: v2.16, until: v2.16}}\n"
	nestedFiles(ws, "v0.5.2", "v0.5.1", "v0.5.1") // main and v2.15 differ; v2.14 agrees

	got, err := holdings(t, ws)

	require.NoError(t, err)
	byB := map[string]HeldValue{}
	for _, h := range got[0].Targets {
		byB[h.Branch] = h.Held[0]
	}
	assert.True(t, byB["main"].Differ)
	assert.Equal(t, "nested modules lag on main", byB["main"].DifferReason)
	assert.True(t, byB["release/v2.15"].Differ)
	assert.Empty(t, byB["release/v2.15"].DifferReason, "the acknowledgement is for v2.16 only")
	assert.False(t, byB["release/v2.14"].Differ)
}

func TestHoldingsOfOneLocationHaveNoLocationsToShow(t *testing.T) {
	got, err := holdings(t, withDevLine(loopFixture(t)))

	require.NoError(t, err)
	for _, h := range got[0].Targets {
		assert.Empty(t, h.Held[0].Locations)
		assert.False(t, h.Held[0].Differ)
	}
}

// twoStreams is a producer with a stream for each of two major versions of its tags,
// and a consumer that holds one for the newer lines and the other for the older.
func twoStreams(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := fixture(t)
	ws.producer = projectSpec{Name: "lib", Repo: "acme/lib", ManagedBy: testTeam, Streams: []extraStream{
		{Name: "one", Pattern: "v1.*", Image: "acme/lib-one"}, {Name: "two", Pattern: "v2.*", Image: "acme/lib-two"},
	}}
	consumer := projectSpec{Name: "svc", Repo: "acme/svc", Lines: []lineSpec{{"v2.16", ""}, {"v2.15", "release/v2.15"}, {"v2.14", "release/v2.14"}}}
	ws.links = []link{
		{Consumer: consumer, Artifact: "lib-one-image", Key: "one", HoldsLines: "{since: v2.15}", Form: "{tag}"},
		{Consumer: consumer, Artifact: "lib-two-image", Key: "two", HoldsLines: "{until: v2.14}", Form: "{tag}"},
	}
	ws.refs = map[string]git.Refs{
		"acme/lib": {Tags: tags("v1.0.0", "v1.1.0", "v2.0.0", "v2.1.0")},
		"acme/svc": {Branches: []git.Branch{{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"}, {Name: "release/v2.14", Commit: "sha-214"}}},
	}
	ws.files = map[string]string{
		"acme/svc@sha-main:build.yaml": "one: v1.1.0\ntwo: v2.0.0\n",
		"acme/svc@sha-215:build.yaml":  "one: v1.0.0\ntwo: v2.0.0\n",
		"acme/svc@sha-214:build.yaml":  "one: v1.0.0\ntwo: v2.0.0\n",
	}
	return ws
}

func TestHoldingsOfAProducerWithSeveralStreamsAreAnEntryForEachStreamOfAConsumer(t *testing.T) {
	got, err := holdings(t, twoStreams(t))

	require.NoError(t, err)
	require.Len(t, got, 2, "svc takes two streams of lib, and each is its own entry")
	one, two := got[0], got[1]
	assert.Equal(t, "lib-one", one.Stream)
	assert.Equal(t, "lib-two", two.Stream)
	assert.Empty(t, one.Error)
	assert.Empty(t, two.Error)

	oneBy := byBranch(t, one)
	require.Len(t, oneBy, 2, "only the branches its holding covers")
	assert.Equal(t, "v1.1.0", oneBy["main"].Stable.Tag, "the latest release of this stream and no other")
	assert.True(t, oneBy["main"].Stable.UpToDate)
	assert.Equal(t, "v1.1.0", oneBy["release/v2.15"].Stable.Tag)
	assert.False(t, oneBy["release/v2.15"].Stable.UpToDate)

	twoBy := byBranch(t, two)
	require.Len(t, twoBy, 1)
	assert.Equal(t, "v2.1.0", twoBy["release/v2.14"].Stable.Tag)
	assert.Equal(t, bump.RelationNewer, twoBy["release/v2.14"].Stable.Relation)
}

func TestSourcesOfAConsumerThatTakesSeveralStreamsOfOneProducerAreAnEntryForEach(t *testing.T) {
	ws := twoStreams(t)

	got, err := Sources(ctx, ws, ws.config().Project("svc"))

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "acme/lib", got[0].Producer)
	assert.Equal(t, "lib-one", got[0].Stream)
	assert.Equal(t, "lib-two", got[1].Stream)
	assert.Empty(t, got[0].Error)
	assert.Empty(t, got[1].Error)
	assert.NotContains(t, got[0].Error+got[1].Error, "one release is of one stream")
}

func TestHoldingsOfAProducerWithOneStreamNameItAndKeepTheirEntries(t *testing.T) {
	got, err := holdings(t, loopFixture(t))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "scc-operator", got[0].Stream)
}
