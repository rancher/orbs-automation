package chain_test

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routeGraph has a producer with two streams, a second producer, and two consumers:
// hold takes both streams of prod and the other producer, hold2 takes one.
func routeGraph(t *testing.T) *chain.Graph {
	return chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		project("prod", "  repo: {name: acme/prod}\n"),
		project("lib", "  repo: {name: acme/lib}\n"),
		project("hold", "  repo: {name: acme/hold}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r/2.15}]\n"),
		project("hold2", "  repo: {name: acme/hold2}\n"),
		stream("prod", "prod", "  lines:\n    - {line: v2.15, series: \"1.x\"}\n    - {line: v2.14, series: \"0.x\"}\n"),
		stream("prod-b", "prod", "  universal: true\n  tagPattern: \"b/v*\"\n"),
		stream("lib", "lib", "  universal: true\n  tagPattern: \"lib/v*\"\n"),
		artifact("img", "prod", "  kind: image\n  name: acme/prod\n"),
		artifact("img2", "prod", "  kind: image\n  name: acme/prod2\n"),
		artifact("imgb", "prod-b", "  kind: image\n  name: acme/prodb\n"),
		artifact("limg", "lib", "  kind: image\n  name: acme/lib\n"),
		holding("h-img", "{project: hold, artifact: img}", holdsImage),
		holding("h-img2", "{project: hold, artifact: img2}", holdsImage),
		holding("h-imgb", "{project: hold, artifact: imgb}", holdsImage),
		holding("h2-img", "{project: hold2, artifact: img}", holdsImage),
		holding("h-limg", "{project: hold, artifact: limg}", holdsImage),
		holding("h-commit", "{project: hold, from: prod}", holdsFrom),
		chaintest.Doc("Edit", "e1", "{holding: [h-img, h-img2]}", "  postEdit:\n    - {run: x}\n"),
	)
}

// shape names a route as consumer/stream and the holdings it has, for comparing.
func shape(routes []chain.Route) []string {
	var out []string
	for _, r := range routes {
		var names []string
		for _, h := range r.Holdings {
			names = append(names, h.Name())
		}
		out = append(out, r.Consumer.Name()+"<-"+r.Producer.Name()+"/"+r.Stream.Name()+": "+strings.Join(names, " "))
	}
	return out
}

func TestTheRoutesOutOfAProducerAreOneForEachConsumerAndStream(t *testing.T) {
	g := routeGraph(t)

	got := shape(g.Project("prod").RoutesFrom())

	assert.Equal(t, []string{
		"hold<-prod/prod: h-img h-img2",
		"hold<-prod/prod-b: h-imgb",
		"hold2<-prod/prod: h2-img",
	}, got, "two artifacts of one stream are one route; a commit holding is none")
	assert.Empty(t, g.Project("hold").RoutesFrom(), "it makes nothing that anything holds")
}

func TestTheRoutesIntoAConsumerAreOneForEachProducerAndStream(t *testing.T) {
	g := routeGraph(t)

	got := shape(g.Project("hold").RoutesInto())

	assert.Equal(t, []string{
		"hold<-prod/prod: h-img h-img2",
		"hold<-prod/prod-b: h-imgb",
		"hold<-lib/lib: h-limg",
	}, got)
	assert.Empty(t, g.Project("prod").RoutesInto())
}

func TestARouteKnowsWhichOfTheConsumersLinesItShips(t *testing.T) {
	g := routeGraph(t)
	routes := g.Project("prod").RoutesFrom()
	require.Len(t, routes, 3)
	toHold := routes[0]

	assert.True(t, toHold.AppliesTo("v2.15"), "the consumer lists it and the stream serves it")
	assert.False(t, toHold.AppliesTo("v2.16"), "the consumer lists it, and the stream does not serve it")
	assert.False(t, toHold.AppliesTo("v2.14"), "the stream serves it, and the consumer does not list it")
}

func TestARouteKnowsTheEditsThatFollowItsHoldingsEachOnce(t *testing.T) {
	g := routeGraph(t)
	routes := g.Project("prod").RoutesFrom()
	require.Len(t, routes, 3)

	edits := routes[0].Edits()

	require.Len(t, edits, 1, "one edit follows both holdings")
	assert.Equal(t, "e1", edits[0].Name())
	assert.Empty(t, routes[1].Edits())
}
