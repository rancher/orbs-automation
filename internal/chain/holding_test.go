package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func holding(name, refs, spec string) string { return chaintest.Doc("Holding", name, refs, spec) }

const (
	at         = "  at: {file: f.yaml, key: .a}\n"
	holdsImage = at + "  holds: {kind: release}\n"
	holdsFrom  = at + "  holds: {kind: commit}\n"
)

// pair is two projects, a producer of an image and a holder of it, each with a stream.
func pair(extra ...string) []string {
	docs := []string{
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		project("prod", "  repo: {name: acme/prod}\n"),
		project("hold", "  repo: {name: acme/hold}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r/2.15}]\n"),
		stream("prod", "prod", "  lines:\n    - {line: v2.15, series: \"1.x\"}\n    - {line: v2.14, series: \"0.x\"}\n"),
		artifact("img", "prod", "  kind: image\n  name: acme/prod\n"),
	}
	return append(docs, extra...)
}

func TestAHoldingKnowsItsHolderProducerAndArtifact(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("holds-img", "{project: hold, artifact: img}", holdsImage),
		holding("holds-commit", "{project: hold, from: prod}", holdsFrom),
	)...)

	require.Len(t, g.Holdings(), 2)
	img, commit := g.Holdings()[0], g.Holdings()[1]
	assert.Equal(t, "holds-img", img.Name())
	assert.Equal(t, "release", img.Kind())
	assert.Equal(t, "hold", img.Holder().Name())
	assert.Equal(t, "prod", img.Producer().Name())
	require.NotNil(t, img.Artifact())
	assert.Equal(t, "img", img.Artifact().Name())

	assert.Equal(t, "commit", commit.Kind())
	assert.Equal(t, "prod", commit.Producer().Name())
	assert.Nil(t, commit.Artifact())
}

func TestAProjectKnowsWhatItHoldsAndWhatIsHeldOfIt(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("holds-img", "{project: hold, artifact: img}", holdsImage),
		holding("holds-commit", "{project: hold, from: prod}", holdsFrom),
	)...)

	require.Len(t, g.Project("hold").Held(), 2)
	assert.Empty(t, g.Project("hold").Sourced())
	require.Len(t, g.Project("prod").Sourced(), 2)
	assert.Equal(t, "holds-img", g.Project("prod").Sourced()[0].Name())
	assert.Empty(t, g.Project("prod").Held())
}

func TestAHoldingAppliesToTheLinesBothProjectsList(t *testing.T) {
	g := chaintest.Load(t, pair(holding("h", "{project: hold, artifact: img}", holdsImage))...)
	h := g.Holdings()[0]

	assert.True(t, h.AppliesTo("v2.15"), "holder and producer both list it")
	assert.False(t, h.AppliesTo("v2.16"), "the producer does not")
	assert.False(t, h.AppliesTo("v2.14"), "the holder does not")
	assert.True(t, h.AppliesTo("2.15"))
}

func TestAHoldingAppliesToItsOwnRangeOfLines(t *testing.T) {
	g := chaintest.Load(t, pair(holding("h", "{project: hold, artifact: img}", at+"  holds: {kind: release, lines: {since: v2.16}}\n"))...)

	assert.False(t, g.Holdings()[0].AppliesTo("v2.15"), "the range leaves it out")
}

func TestAHoldingOfASingleLayoutHolderAndAUniversalStream(t *testing.T) {
	g := chaintest.Load(t,
		project("prod", "  repo: {name: acme/prod}\n"),
		project("hold", "  repo: {name: acme/hold}\n"),
		stream("prod", "prod", "  universal: true\n"),
		artifact("img", "prod", "  kind: image\n  name: acme/prod\n"),
		holding("h", "{project: hold, artifact: img}", holdsImage),
	)

	assert.True(t, g.Holdings()[0].AppliesTo("v2.16"))
	assert.True(t, g.Holdings()[0].AppliesTo("v9.9"))
}

func TestAHoldingOfAnArtifactAppliesOnlyToTheLinesTheArtifactIsFor(t *testing.T) {
	g := chaintest.Load(t, pair(
		artifact("late", "prod", "  kind: image\n  name: acme/late\n  lines: {since: v2.16}\n"),
		holding("h", "{project: hold, artifact: late}", holdsImage),
	)...)

	assert.False(t, g.Holdings()[0].AppliesTo("v2.15"))
}

func TestAHoldingOfACommitAppliesToTheLinesTheProducerHas(t *testing.T) {
	g := chaintest.Load(t, pair(holding("h", "{project: hold, from: prod}", holdsFrom))...)
	h := g.Holdings()[0]

	assert.True(t, h.AppliesTo("v2.15"))
	assert.False(t, h.AppliesTo("v2.16"), "no stream of the producer serves it")
}

func TestAnEditFollowsHoldingsAndHasStepsForALine(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("h1", "{project: hold, artifact: img}", holdsImage),
		holding("h2", "{project: hold, from: prod}", holdsFrom),
		chaintest.Doc("Edit", "regen", "{holding: [h1, h2]}", "  preEdit:\n    - {run: [make, prep]}\n  postEdit:\n    - {run: gen}\n    - {run: old, until: v2.14}\n    - {run: new, since: v2.16}\n"),
		chaintest.Doc("Edit", "bump", "{holding: h1}", "  strategy: chartsPackageBump\n  with: {package: x}\n"),
	)...)
	h1, h2 := g.Holdings()[0], g.Holdings()[1]

	require.Len(t, h1.Edits(), 2)
	assert.Equal(t, "regen", h1.Edits()[0].Name())
	assert.Equal(t, "bump", h1.Edits()[1].Name())
	require.Len(t, h2.Edits(), 1)
	assert.Equal(t, []string{"h1", "h2"}, []string{h1.Edits()[0].Holdings()[0].Name(), h1.Edits()[0].Holdings()[1].Name()})

	bump := h1.Edits()[1]
	assert.Equal(t, "chartsPackageBump", bump.Strategy())
	assert.Equal(t, map[string]string{"package": "x"}, bump.With())

	pre, post := h1.Edits()[0].Steps("v2.15")
	require.Len(t, pre, 1)
	assert.Equal(t, []string{"make", "prep"}, []string(pre[0].Run))
	require.Len(t, post, 1)
	assert.Equal(t, []string{"gen"}, []string(post[0].Run))

	_, post = h1.Edits()[0].Steps("v2.16")
	require.Len(t, post, 2)
	assert.Equal(t, []string{"new"}, []string(post[1].Run))
	_, post = h1.Edits()[0].Steps("v2.14")
	require.Len(t, post, 2)
	assert.Equal(t, []string{"old"}, []string(post[1].Run))
}

func TestAHoldingSaysWhetherItsDocumentAcknowledgesAWarning(t *testing.T) {
	holdingDoc := "apiVersion: gero/v1alpha2\nkind: Holding\nname: h\nrefs: {project: cons, artifact: a}\n" +
		"acknowledge:\n" +
		"  - {rule: locations-differ, reason: nested modules lag, lines: {since: v2.15, until: v2.16}}\n" +
		"  - {rule: circular-holding, reason: they build each other}\n" +
		"spec:\n  at: {file: f.yaml, key: .a}\n  holds: {kind: release}\n"
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", "  universal: true\n"),
		chaintest.Doc("Artifact", "a", "{stream: prod}", "  kind: image\n  name: acme/prod\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n"),
		holdingDoc,
	)
	h := g.Holdings()[0]

	reason, ok := h.Acknowledgement("locations-differ", "v2.16")
	assert.True(t, ok)
	assert.Equal(t, "nested modules lag", reason)

	_, ok = h.Acknowledgement("locations-differ", "v2.14")
	assert.False(t, ok, "v2.14 is outside the lines of the acknowledgement")

	reason, ok = h.Acknowledgement("circular-holding", "")
	assert.True(t, ok)
	assert.Equal(t, "they build each other", reason)

	_, ok = h.Acknowledgement("stream-differs-from-destination", "v2.16")
	assert.False(t, ok)
}

func TestAHoldingSaysWhatItHoldsAndWhereItIsLookedFor(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("full", "{project: hold, artifact: img}",
			"  at:\n    - {file: a.yaml, key: .a, format: yaml, lines: {since: v2.15}}\n    - {file: b.yaml, key: .b, format: dockerfile}\n"+
				"  holds: {kind: release, form: \"{tag}\", accept: all, lines: {until: v2.15}}\n"),
		holding("plain", "{project: hold, artifact: img}", holdsImage),
	)...)
	full, plain := g.Holdings()[0], g.Holdings()[1]

	assert.Equal(t, "{tag}", full.Form())
	assert.True(t, full.AcceptsAll())
	assert.False(t, plain.AcceptsAll(), "a holding takes stable releases unless it says otherwise")
	assert.True(t, full.HoldsLine("v2.15"))
	assert.True(t, full.HoldsLine("2.14"), "with or without the v")
	assert.False(t, full.HoldsLine("v2.16"), "its own range ends at v2.15")
	assert.True(t, plain.HoldsLine("v2.16"), "no range holds every line")

	locations := full.Locations()
	require.Len(t, locations, 2)
	assert.Equal(t, "a.yaml", locations[0].File)
	assert.Equal(t, ".a", locations[0].Key)
	assert.Equal(t, "yaml", locations[0].Format)
	assert.Equal(t, "dockerfile", locations[1].Format)
	assert.True(t, locations[0].AppliesTo("v2.15"))
	assert.False(t, locations[0].AppliesTo("v2.14"), "its file exists from v2.15")
	assert.True(t, locations[1].AppliesTo("v2.14"), "no range is every line")
	assert.Empty(t, full.Stage())
}

func TestAHoldingKnowsItsStage(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("staged", "{project: hold, artifact: img}", at+"  holds: {kind: release, stage: dev}\n"),
	)...)

	assert.Equal(t, "dev", g.Holdings()[0].Stage())
}

func TestAnArtifactSaysWhetherItIsForSomeLinesOnly(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("prod", "  repo: {name: acme/prod}\n"),
		stream("s", "prod", "  universal: true\n"),
		artifact("all", "s", "  kind: image\n  name: acme/all\n"),
		artifact("some", "s", "  kind: image\n  name: acme/some\n  lines: {since: v2.16}\n"),
	)
	arts := g.Project("prod").Artifacts()
	require.Len(t, arts, 2)

	assert.False(t, arts[0].Ranged())
	assert.True(t, arts[1].Ranged())
}

func TestAProjectKnowsWhetherItFindsItsLinesInItsTags(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("tags", "  repo: {name: acme/tags}\n"),
		project("bare", "  repo: {name: acme/bare}\n"),
		project("branches", "  repo: {name: acme/branches}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r/2.15}]\n"),
		stream("tags-one", "tags", "  lines:\n    - {line: v2.16, series: \"1.x\"}\n"),
		stream("branches-one", "branches", "  lines:\n    - {line: v2.16, series: \"1.x\"}\n"),
	)

	assert.True(t, g.Project("tags").LinesInTags(), "one branch, and a stream that lists lines")
	assert.False(t, g.Project("bare").LinesInTags(), "one branch and no lines")
	assert.False(t, g.Project("branches").LinesInTags(), "a branch for each line, whatever its streams list")
	for _, p := range g.Projects() {
		rows := p.ReleaseTargets()
		if p.LinesInTags() {
			assert.NotEmpty(t, rows[0].Line, p.Name(), ": a row for each line")
		}
	}
}
