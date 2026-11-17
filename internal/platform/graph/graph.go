package graph

// Edge is one edge out of a node.
type Edge[N comparable, E any] struct {
	From, To N
	Payload  E
}

// Graph is a directed graph of nodes N and edge payloads E.
type Graph[N comparable, E any] struct{}

// New returns an empty graph.
func New[N comparable, E any]() *Graph[N, E] { return &Graph[N, E]{} }

// AddNode adds a node with no edge. A node that is already there keeps its place.
func (g *Graph[N, E]) AddNode(n N) {}

// AddEdge adds an edge, and the nodes at its ends when they are new. Two edges
// between the same nodes are both kept.
func (g *Graph[N, E]) AddEdge(from, to N, payload E) {}

// Nodes returns every node in the order added.
func (g *Graph[N, E]) Nodes() []N { return nil }

// Edges returns every edge, in the order added.
func (g *Graph[N, E]) Edges() []Edge[N, E] { return nil }

// Out returns the edges out of a node, in the order added.
func (g *Graph[N, E]) Out(n N) []Edge[N, E] { return nil }

// Cycles returns one cycle for each set of nodes that can all reach one another,
// and for each node that points at itself. A cycle is the path of the shortest
// way back to the first node of its set, that node first and without repeating
// it at the end. Cycles come in the order of the first node of each.
func (g *Graph[N, E]) Cycles() [][]N { return nil }

// Reachable returns every node a path from a node reaches, in the order a
// breadth-first walk finds them. The node itself is there only when it is on a
// cycle.
func (g *Graph[N, E]) Reachable(from N) []N { return nil }

// Path returns the shortest path from one node to another, both included, and
// false when there is none. A node has the path of itself alone to itself.
func (g *Graph[N, E]) Path(from, to N) ([]N, bool) { return nil, false }
