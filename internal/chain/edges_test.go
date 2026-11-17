package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ring is the pinWorld with a Holding of BRO's own module in wrangler, so that BRO
// and wrangler hold something of each other.
func ring() []string {
	return pinWorld(
		stream("bro", "bro", "  universal: true\n"),
		chaintest.Doc("Artifact", "bro-mod", "{stream: bro}", "  kind: gomod\n  name: github.com/acme/bro\n"),
		holding("wr-holds-bro", "{project: wr, artifact: bro-mod}", "  at: {file: go.mod, key: github.com/acme/bro, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: stable}\n"),
	)
}

func ids(edges []chain.Edge) []string {
	var out []string
	for _, e := range edges {
		out = append(out, string(e.Kind)+" "+string(e.From)+" "+string(e.To))
	}
	return out
}

func TestTheHoldsEdgesAreTheHoldingsWrittenThenThoseThePinsMake(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	edges := g.Edges(chain.EdgeHolds)

	assert.Equal(t, []string{"holds project:bro project:wr", "holds project:bro project:la"}, ids(edges))
	assert.Equal(t, "bro-holds-wr", edges[0].Holding.Name())
	assert.False(t, edges[0].Holding.Virtual())
	assert.True(t, edges[1].Holding.Virtual())
}

func TestThePinsEdgesAreFromThePinningArtifactToThePinned(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	assert.Equal(t, []string{"pins artifact:wr-mod artifact:la-mod"}, ids(g.Edges(chain.EdgePins)))
}

func TestTheFollowsEdgesAreFromAnEditToEachHoldingItFollows(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	edges := g.Edges(chain.EdgeFollows)

	assert.Equal(t, []string{
		"follows edit:bro-bump holding:bro-holds-wr",
		"follows edit:bro-bump holding:bro-holds-wr+la-mod",
	}, ids(edges), "the virtual Holding a pin makes is followed too")
	assert.Equal(t, "bro-bump", edges[0].Edit.Name())
	assert.Equal(t, "bro-holds-wr", edges[0].Holding.Name())
}

func TestEdgesOfNoKindNamedAreAllOfThemInTheOrderOfTheKinds(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	all := ids(g.Edges())

	assert.Len(t, all, 5)
	assert.Equal(t, all, append(append(ids(g.Edges(chain.EdgeHolds)), ids(g.Edges(chain.EdgePins))...), ids(g.Edges(chain.EdgeFollows))...))
	assert.Equal(t, ids(g.Edges(chain.EdgeHolds, chain.EdgePins)), all[:3])
}

func TestTheRoutesAgreeWithTheHoldsEdges(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	var fromRoutes []string
	for _, p := range g.Projects() {
		for _, r := range p.RoutesFrom() {
			for _, h := range r.Holdings {
				fromRoutes = append(fromRoutes, string(chain.ProjectID(r.Consumer))+" "+string(chain.ProjectID(r.Producer))+" "+h.Name())
			}
		}
	}
	var fromEdges []string
	for _, e := range g.Edges(chain.EdgeHolds) {
		if e.Holding.Kind() == chain.HoldsRelease {
			fromEdges = append(fromEdges, string(e.From)+" "+string(e.To)+" "+e.Holding.Name())
		}
	}

	assert.ElementsMatch(t, fromEdges, fromRoutes)
}

func TestAHoldingRingOfTwoIsOneCycle(t *testing.T) {
	g := chaintest.Load(t, ring()...)

	assert.Equal(t, [][]chain.NodeID{{"project:bro", "project:wr"}}, g.Cycles(chain.EdgeHolds))
	assert.Empty(t, chaintest.Load(t, pinWorld()...).Cycles(chain.EdgeHolds))
	assert.Empty(t, g.Cycles(chain.EdgePins))
}

func TestWhatAProjectReachesIsWhatItHoldsAndWhatThoseHold(t *testing.T) {
	g := chaintest.Load(t, ring()...)

	assert.Equal(t, []chain.NodeID{"project:wr", "project:la", "project:bro"}, g.Reachable("project:bro", chain.EdgeHolds))
	assert.Equal(t, []chain.NodeID{"artifact:la-mod"}, g.Reachable("artifact:wr-mod", chain.EdgePins))
	assert.Empty(t, g.Reachable("project:la", chain.EdgeHolds))
}

func TestAPathFollowsTheKindsAskedFor(t *testing.T) {
	g := chaintest.Load(t, ring()...)

	p, ok := g.Path("project:wr", "project:la", chain.EdgeHolds)
	require.True(t, ok)
	assert.Equal(t, []chain.NodeID{"project:wr", "project:bro", "project:la"}, p)
	_, ok = g.Path("project:la", "project:wr", chain.EdgeHolds)
	assert.False(t, ok)
}

func TestNodeIDsNameTheKindAndTheDocument(t *testing.T) {
	g := chaintest.Load(t, pinWorld()...)

	bro := g.Project("bro")
	assert.Equal(t, chain.NodeID("project:bro"), chain.ProjectID(bro))
	assert.Equal(t, chain.NodeID("holding:bro-holds-wr"), chain.HoldingID(bro.Held()[0]))
	assert.Equal(t, chain.NodeID("artifact:wr-mod"), chain.ArtifactID(g.Project("wr").Artifacts()[0]))
	assert.Equal(t, chain.NodeID("edit:bro-bump"), chain.EditID(bro.Held()[0].Edits()[0]))
}
