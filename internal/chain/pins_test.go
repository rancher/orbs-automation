package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinWorld is wrangler (wr) and lasso (la), each a Go module, wrangler's pinning
// lasso's, and BRO (bro), which holds wrangler for v2.15 and newer in go.mod and in
// cmd/tool/go.mod from v2.14, and has an Edit that follows that Holding.
func pinWorld(extra ...string) []string {
	const hold = "  at:\n    - {file: go.mod, key: github.com/acme/wr, format: gomod}\n    - {file: cmd/tool/go.mod, key: github.com/acme/wr, format: gomod, lines: {since: v2.16}}\n  holds: {kind: release, form: \"{bareTag}\", accept: stable, lines: {since: v2.15}}\n"
	docs := []string{
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		project("wr", "  repo: {name: acme/wr, managedBy: frameworks}\n"),
		project("la", "  repo: {name: acme/la, managedBy: frameworks}\n"),
		project("bro", "  repo: {name: acme/bro, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}, {line: v2.14, branch: r14}]\n"),
		stream("wr", "wr", "  universal: true\n"),
		stream("la", "la", "  universal: true\n"),
		artifact("la-mod", "la", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		holding("bro-holds-wr", "{project: bro, artifact: wr-mod}", hold),
		chaintest.Doc("Edit", "bro-bump", "{holding: bro-holds-wr}", "  strategy: goBump\n"),
	}
	return append(docs, extra...)
}

func TestAnArtifactKnowsWhatItPinsAndWhatPinsIt(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	wr, la := g.Project("wr").Artifacts()[0], g.Project("la").Artifacts()[0]
	require.Len(t, wr.Pins(), 1)
	assert.Equal(t, "la-mod", wr.Pins()[0].Name())
	assert.Equal(t, "go.mod", wr.PinFile(), "the default is filled")
	assert.Empty(t, la.Pins())
	require.Len(t, la.PinnedBy(), 1)
	assert.Equal(t, "wr-mod", la.PinnedBy()[0].Name())
	assert.Empty(t, wr.PinnedBy())
}

func TestAPinMakesAVirtualHoldingForEachHoldingOfThePinningArtifact(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	require.Len(t, g.Holdings(), 1, "the documents only")
	pinning := g.Holdings()[0]
	require.Len(t, g.Virtual(), 1)
	v := g.Virtual()[0]
	assert.True(t, v.Virtual())
	assert.False(t, pinning.Virtual())
	assert.Equal(t, "bro-holds-wr+la-mod", v.Name())
	assert.Equal(t, "bro", v.Holder().Name(), "the consumer's")
	assert.Equal(t, "la", v.Producer().Name(), "the pinned module's project")
	require.NotNil(t, v.Artifact())
	assert.Equal(t, "la-mod", v.Artifact().Name())
	assert.Equal(t, "release", v.Kind())
	assert.Same(t, pinning, v.PinnedBy())
	assert.Equal(t, []string{"bro-holds-wr+la-mod"}, holdingNames(pinning.Pinned()))
	assert.Nil(t, pinning.PinnedBy())
}

func TestAVirtualHoldingHasTheFilesAndLinesOfTheHoldingItComesFrom(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	v := g.Virtual()[0]
	require.Len(t, v.Doc.Spec.At, 2)
	assert.Equal(t, "go.mod", v.Doc.Spec.At[0].File)
	assert.Equal(t, "github.com/acme/la", v.Doc.Spec.At[0].Key, "the pinned module's own path")
	assert.Equal(t, "gomod", v.Doc.Spec.At[0].Format)
	assert.Equal(t, "cmd/tool/go.mod", v.Doc.Spec.At[1].File)
	assert.Equal(t, "v2.16", v.Doc.Spec.At[1].Lines.Since, "a location's own lines")
	assert.Equal(t, "{bareTag}", v.Doc.Spec.Holds.Form)
	assert.Equal(t, "all", v.Doc.Spec.Holds.Accept, "a pin is a floor, so what the consumer holds may be a prerelease")
	assert.Equal(t, "v2.15", v.Doc.Spec.Holds.Lines.Since)
	assert.True(t, v.AppliesTo("v2.16"))
	assert.True(t, v.AppliesTo("v2.15"))
	assert.False(t, v.AppliesTo("v2.14"))
}

func TestAVirtualHoldingIsInTheProjectsThatHoldAndMakeIt(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	v := g.Virtual()[0]
	assert.Contains(t, g.Project("bro").Held(), v)
	assert.Contains(t, g.Project("la").Sourced(), v, "lasso is shipped to bro through it")
	assert.NotContains(t, g.Project("wr").Sourced(), v)
}

func TestAVirtualHoldingFollowsTheEditsOfTheHoldingItComesFrom(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	pinning, v := g.Holdings()[0], g.Virtual()[0]
	require.Len(t, v.Edits(), 1)
	assert.Same(t, pinning.Edits()[0], v.Edits()[0])
	assert.Equal(t, []string{"bro-holds-wr", "bro-holds-wr+la-mod"}, holdingNames(v.Edits()[0].Holdings()), "so a goBump sees the file the virtual Holding wrote")
}

func TestAnArtifactThatPinsNothingMakesNoVirtualHolding(t *testing.T) {
	g := chaintest.Load(t, pair(holding("holds-img", "{project: hold, artifact: img}", holdsImage))...)

	assert.Empty(t, g.Virtual())
}

func TestTwoPinnedArtifactsMakeAVirtualHoldingEach(t *testing.T) {
	g := chaintest.Load(t, pinWorld(
		artifact("la-other", "la", "  kind: gomod\n  name: github.com/acme/lb\n"),
		chaintest.Doc("Artifact", "wr2-mod", "{stream: wr, pins: [la-mod, la-other]}", "  kind: gomod\n  name: github.com/acme/wr2\n"),
		holding("bro-holds-wr2", "{project: bro, artifact: wr2-mod}", "  at: {file: tool/go.mod, key: github.com/acme/wr2, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", lines: {until: v2.14}}\n"),
	)...)

	assert.Equal(t, []string{"bro-holds-wr+la-mod", "bro-holds-wr2+la-mod", "bro-holds-wr2+la-other"}, holdingNames(g.Virtual()))
}

func holdingNames(hs []*chain.Holding) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.Name()
	}
	return out
}
