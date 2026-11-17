package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func build(pairs ...string) *Graph[string, int] {
	g := New[string, int]()
	for i := 0; i+1 < len(pairs); i += 2 {
		g.AddEdge(pairs[i], pairs[i+1], i/2)
	}
	return g
}

func TestNodesAndEdgesComeInTheOrderAdded(t *testing.T) {
	g := New[string, int]()
	g.AddNode("z")
	g.AddEdge("b", "a", 7)
	g.AddNode("z")
	g.AddEdge("b", "a", 8)
	assert.Equal(t, []string{"z", "b", "a"}, g.Nodes())
	assert.Equal(t, []Edge[string, int]{{"b", "a", 7}, {"b", "a", 8}}, g.Edges())
	assert.Equal(t, g.Edges(), g.Out("b"))
	assert.Empty(t, g.Out("a"))
	assert.Empty(t, g.Out("missing"))
}

func TestCycles(t *testing.T) {
	tests := []struct {
		name  string
		graph *Graph[string, int]
		want  [][]string
	}{
		{"empty", New[string, int](), nil},
		{"a line has none", build("a", "b", "b", "c"), nil},
		{"a diamond is not a cycle", build("a", "b", "a", "c", "b", "d", "c", "d"), nil},
		{"a pair", build("a", "b", "b", "a"), [][]string{{"a", "b"}}},
		{"a ring of three", build("a", "b", "b", "c", "c", "a"), [][]string{{"a", "b", "c"}}},
		{"a node that points at itself", build("a", "a"), [][]string{{"a"}}},
		{"the cycle starts at the first node added", build("x", "b", "b", "c", "c", "b", "b", "x"), [][]string{{"x", "b"}}},
		{"two apart", build("a", "b", "b", "a", "c", "d", "d", "e", "e", "c"), [][]string{{"a", "b"}, {"c", "d", "e"}}},
		{"one for a set with two ways round", build("a", "b", "b", "a", "b", "c", "c", "a"), [][]string{{"a", "b"}}},
		{"a tail into a cycle is not part of it", build("t", "a", "a", "b", "b", "a"), [][]string{{"a", "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.graph.Cycles())
		})
	}
}

func TestReachable(t *testing.T) {
	g := build("a", "b", "a", "c", "b", "d", "c", "d", "d", "e", "x", "a")
	assert.Equal(t, []string{"b", "c", "d", "e"}, g.Reachable("a"))
	assert.Empty(t, g.Reachable("e"))
	assert.Empty(t, g.Reachable("missing"))
}

func TestReachableHasTheNodeOnlyOnACycle(t *testing.T) {
	assert.Equal(t, []string{"b", "a"}, build("a", "b", "b", "a").Reachable("a"))
	assert.Equal(t, []string{"a"}, build("a", "a").Reachable("a"))
}

func TestPath(t *testing.T) {
	g := build("a", "b", "b", "c", "c", "d", "a", "d", "d", "a")
	p, ok := g.Path("a", "d")
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "d"}, p, "the shortest")
	p, ok = g.Path("b", "a")
	assert.True(t, ok)
	assert.Equal(t, []string{"b", "c", "d", "a"}, p)
	p, ok = g.Path("c", "c")
	assert.True(t, ok)
	assert.Equal(t, []string{"c"}, p)
	_, ok = g.Path("a", "missing")
	assert.False(t, ok)
	_, ok = build("a", "b").Path("b", "a")
	assert.False(t, ok)
}

func TestQueriesAreTheSameOnEveryRun(t *testing.T) {
	for range 20 {
		g := build("a", "b", "b", "c", "c", "a", "c", "d", "d", "b")
		assert.Equal(t, [][]string{{"a", "b", "c"}}, g.Cycles())
		assert.Equal(t, []string{"b", "c", "a", "d"}, g.Reachable("a"))
	}
}
