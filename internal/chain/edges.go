package chain

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
func ProjectID(p *Project) NodeID { return "" }

// ArtifactID is the node of an Artifact.
func ArtifactID(a *Artifact) NodeID { return "" }

// HoldingID is the node of a Holding.
func HoldingID(h *Holding) NodeID { return "" }

// EditID is the node of an Edit.
func EditID(e *Edit) NodeID { return "" }

// Edges returns the edges of the kinds, or of every kind when none is named.
func (g *Graph) Edges(kinds ...EdgeKind) []Edge { return nil }

// Cycles returns a cycle for each set of Projects (for holds) or Artifacts (for
// pins) that all reach one another.
func (g *Graph) Cycles(kind EdgeKind) [][]NodeID { return nil }

// Reachable returns the nodes a path along edges of the kinds reaches from a node.
func (g *Graph) Reachable(from NodeID, kinds ...EdgeKind) []NodeID { return nil }

// Path returns the shortest path along edges of the kinds from one node to another.
func (g *Graph) Path(from, to NodeID, kinds ...EdgeKind) ([]NodeID, bool) { return nil, false }
