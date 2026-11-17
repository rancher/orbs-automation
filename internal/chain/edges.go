package chain

import (
	"slices"

	"github.com/rancher/orbs-automation/internal/platform/graph"
)

// NodeID names a node of the graph as kind:name, such as project:rancher.
type NodeID string

// EdgeKind is what an edge says.
type EdgeKind string

// The kinds of edge.
const (
	EdgeHolds   EdgeKind = "holds"   // holder Project to producer Project, made by a Holding
	EdgePins    EdgeKind = "pins"    // pinning Artifact to pinned Artifact
	EdgeFollows EdgeKind = "follows" // Edit to the Holding it follows
)

// Edge is one edge of the graph, with the documents it was made from.
type Edge struct {
	Kind     EdgeKind
	From, To NodeID
	Holding  *Holding // the Holding that holds, or that is followed
	Edit     *Edit    // the Edit that follows
}

// ProjectID is the node of a Project.
func ProjectID(p *Project) NodeID { return NodeID("project:" + p.Name()) }

// ArtifactID is the node of an Artifact.
func ArtifactID(a *Artifact) NodeID { return NodeID("artifact:" + a.Name()) }

// HoldingID is the node of a Holding.
func HoldingID(h *Holding) NodeID { return NodeID("holding:" + h.Name()) }

// EditID is the node of an Edit.
func EditID(e *Edit) NodeID { return NodeID("edit:" + e.Name()) }

// Edges returns the edges of the kinds, or of every kind when none is named. The
// holds edges come first, the Holdings written in the order read and then the
// virtual ones the pins make; then the pins edges, by Project and its Artifacts; then
// the follows edges, by Edit in the order read. It builds nothing of its own: it reads
// the links Build made.
func (g *Graph) Edges(kinds ...EdgeKind) []Edge {
	want := func(k EdgeKind) bool { return len(kinds) == 0 || slices.Contains(kinds, k) }
	var out []Edge
	if want(EdgeHolds) {
		for _, h := range slices.Concat(g.holdings, g.virtual) {
			out = append(out, Edge{Kind: EdgeHolds, From: ProjectID(h.holder), To: ProjectID(h.producer), Holding: h})
		}
	}
	if want(EdgePins) {
		for _, p := range g.projects {
			for _, a := range p.artifacts {
				for _, pinned := range a.pins {
					out = append(out, Edge{Kind: EdgePins, From: ArtifactID(a), To: ArtifactID(pinned)})
				}
			}
		}
	}
	if want(EdgeFollows) {
		for _, e := range g.edits {
			for _, h := range e.holdings {
				out = append(out, Edge{Kind: EdgeFollows, From: EditID(e), To: HoldingID(h), Holding: h, Edit: e})
			}
		}
	}
	return out
}

// graphOf is the edges of the kinds as a graph the queries of internal/platform/graph
// are asked of.
func (g *Graph) graphOf(kinds ...EdgeKind) *graph.Graph[NodeID, Edge] {
	out := graph.New[NodeID, Edge]()
	for _, e := range g.Edges(kinds...) {
		out.AddEdge(e.From, e.To, e)
	}
	return out
}

// Cycles returns a cycle for each set of Projects (for holds) or Artifacts (for
// pins) that all reach one another, as internal/platform/graph gives it: the
// shortest way round from the first of the set.
func (g *Graph) Cycles(kind EdgeKind) [][]NodeID { return g.graphOf(kind).Cycles() }

// Reachable returns the nodes a path along edges of the kinds reaches from a node,
// the node itself only when it is on a cycle.
func (g *Graph) Reachable(from NodeID, kinds ...EdgeKind) []NodeID {
	return g.graphOf(kinds...).Reachable(from)
}

// Path returns the shortest path along edges of the kinds from one node to another,
// both included, and false when there is none.
func (g *Graph) Path(from, to NodeID, kinds ...EdgeKind) ([]NodeID, bool) {
	return g.graphOf(kinds...).Path(from, to)
}
