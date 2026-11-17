// Package graphresolve holds the logic of gero config resolve graph: it draws
// the whole central config as a D2 diagram, each Project a node and each holding
// a node between the Project that produces and the one that holds. It is
// framework-free: cmd/ owns the cobra wiring.
package graphresolve

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Graph needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Chain() (*chain.Graph, error)
}

// Kind is the kind of diagram to draw.
type Kind string

const (
	// KindD2 is the flow diagram, written as D2.
	KindD2 Kind = "d2"
	// KindMindmap is the mind map, written as Mermaid.
	KindMindmap Kind = "mindmap"
)

// Kinds lists the kinds Graph can draw, the default first.
var Kinds = []Kind{KindD2, KindMindmap}

// Graph draws the central config found from s and the working directory cwd as
// a diagram of the kind asks for.
func Graph(s clicfg.Settings, cwd string, kind Kind) (string, error) {
	return run(workspace.Open(s, cwd), s.Output, kind)
}

// run draws the config as the output format and the kind ask for.
func run(ws Workspace, format string, kind Kind) (string, error) {
	if format != clicfg.OutputText {
		return "", fmt.Errorf("output format %q is not supported by config resolve graph: it writes D2 or Mermaid", format)
	}
	switch kind {
	case KindD2:
		return graph(ws)
	case KindMindmap:
		return mindmap(ws)
	}
	return "", fmt.Errorf("graph type %q is not supported by config resolve graph", kind)
}

// mindmap draws the config as a Mermaid mind map. A mind map is a tree and has
// no edges, so a Project is a branch of the root, and what joins two Projects is
// said on both of them: the artifact says where it reaches, and the holder says
// what it holds and from whom.
func mindmap(ws Workspace) (string, error) {
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	holdings := holdingsOf(g)
	projects := slices.Clone(g.Projects())
	slices.SortFunc(projects, func(a, c *chain.Project) int { return strings.Compare(a.Slug(), c.Slug()) })

	m := &mindmapWriter{}
	m.b.WriteString("mindmap\n")
	m.node(1, `root(("gero config"))`)
	for _, p := range projects {
		m.project(g, p, holdings)
	}
	return m.b.String(), nil
}

type mindmapWriter struct {
	b strings.Builder
	n int
}

// node writes a line at depth, which is the indentation in steps of two spaces.
func (m *mindmapWriter) node(depth int, text string) {
	m.b.WriteString(strings.Repeat("  ", depth) + text + "\n")
}

// label writes text as a node of its own, with an id no other node has.
func (m *mindmapWriter) label(depth int, text string) {
	m.n++
	text = strings.NewReplacer(`"`, "'", "\n", " ").Replace(text)
	m.node(depth, fmt.Sprintf(`n%d["%s"]`, m.n, text))
}

func (m *mindmapWriter) project(g *chain.Graph, p *chain.Project, holdings []*chain.Holding) {
	var marks []string
	switch {
	case p.External():
		marks = append(marks, classExternal)
	case p.Shared():
		marks = append(marks, classShared)
	case p.ManagedBy() != "":
		marks = append(marks, "managed by "+p.ManagedBy())
	}
	if p.FinalDestination() {
		marks = append(marks, classHub)
	}
	text := p.Name()
	if len(marks) > 0 {
		text += " (" + strings.Join(marks, ", ") + ")"
	}
	m.label(2, text)
	for _, st := range p.Streams() {
		m.label(3, streamLabel(st))
		for _, a := range st.Artifacts() {
			m.label(4, "artifact "+a.Name()+": "+a.Ref())
			for _, h := range holdings {
				if h.Artifact() == a {
					m.label(5, reaches(g, h))
				}
			}
		}
	}
	for _, h := range holdings {
		if h.Producer() == p && h.Artifact() == nil {
			m.label(3, reaches(g, h))
		}
	}
	first := true
	for _, h := range holdings {
		if h.Holder() != p {
			continue
		}
		if first {
			m.label(3, "holds")
			first = false
		}
		kind := h.Kind()
		if h.Virtual() {
			kind += ", pinned"
		}
		m.label(4, fmt.Sprintf("%s: %s from %s", h.Name(), kind, h.Producer().Name()))
		for _, e := range h.Edits() {
			m.label(5, "edit "+editLabel(e))
		}
	}
}

// reaches says where a holding takes what its producer makes: its holder, by
// the holding, for the lines it serves.
func reaches(g *chain.Graph, h *chain.Holding) string {
	text := "to " + h.Holder().Name() + " via " + h.Name()
	var served []string
	for _, l := range g.Lines() {
		if h.AppliesTo(l.ID) {
			served = append(served, l.ID)
		}
	}
	if len(served) > 0 {
		text += " (" + strings.Join(served, ", ") + ")"
	}
	return text
}

// The classes of a node. A Project has one of managed, shared and external, and
// hub as well when it is where releases end.
const (
	classManaged  = "managed"
	classShared   = "shared"
	classExternal = "external"
	classHub      = "hub"
	classHolding  = "holding"
	classStream   = "stream"
	classArtifact = "artifact"
	classEdit     = "edit"
)

const header = `direction: down

vars: {
  d2-config: {layout-engine: elk}
}

classes: {
  managed: {style.fill: "#d8f0e0"}
  shared: {style.fill: "#fdf0c8"}
  external: {style.fill: "#eeeeee"; style.stroke-dash: 3}
  hub: {style.stroke-width: 4}
  holding: {shape: hexagon; style.font-size: 12}
  stream: {style.fill: "#ffffff"; style.stroke-dash: 2}
  artifact: {shape: cylinder}
  edit: {style.fill: "#fde2e2"}
}
`

func graph(ws Workspace) (string, error) {
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(header)

	projects := slices.Clone(g.Projects())
	slices.SortFunc(projects, func(a, c *chain.Project) int { return strings.Compare(a.Slug(), c.Slug()) })
	b.WriteString("\n")
	for _, p := range projects {
		writeProject(&b, p)
	}

	// What a pin makes is a holding of its holder as well: a consumer says nothing of
	// lasso where it holds wrangler, and lasso still flows to it.
	holdings := holdingsOf(g)
	b.WriteString("\n")
	for _, h := range holdings {
		writeHolding(&b, g, h)
	}
	follows := g.Edges(chain.EdgeFollows)
	slices.SortStableFunc(follows, func(a, c chain.Edge) int { return strings.Compare(a.Holding.Name(), c.Holding.Name()) })
	edits := map[string]*chain.Edit{}
	for _, f := range follows {
		edits[f.Edit.Name()] = f.Edit
	}

	b.WriteString("\n")
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		fmt.Fprintf(&b, "%s: {class: [%s]; label: %s}\n", quote(editKey(edits[name])), classEdit, quote(editLabel(edits[name])))
	}
	for _, f := range follows { // a follows edge points at the Holding; the diagram flows from it
		fmt.Fprintf(&b, "%s -> %s: {style.stroke-dash: 4}\n", quote(string(f.To)), quote(string(f.From)))
	}
	return b.String(), nil
}

func writeProject(b *strings.Builder, p *chain.Project) {
	class, label := classManaged, p.Name()
	switch {
	case p.External():
		class = classExternal
	case p.Shared():
		class = classShared
	case p.ManagedBy() != "":
		label += "\nmanaged by " + p.ManagedBy()
	}
	classes := class
	if p.FinalDestination() {
		classes += "; " + classHub
	}
	fmt.Fprintf(b, "%s: {\n  class: [%s]\n  label: %s\n", quote(p.Slug()), classes, quote(label))
	for _, st := range p.Streams() {
		fmt.Fprintf(b, "  %s: {\n    class: [%s]\n    label: %s\n", quote(streamKey(st)), classStream, quote(streamLabel(st)))
		for _, a := range st.Artifacts() {
			fmt.Fprintf(b, "    %s: {class: [%s]; label: %s}\n", quote(artifactKey(a)), classArtifact, quote(a.Name()+"\n"+a.Ref()))
		}
		b.WriteString("  }\n")
	}
	b.WriteString("}\n")
}

// streamLabel names the stream and, when it has one, its tag pattern, since a
// project may release several streams of one thing that only the pattern tells
// apart.
func streamLabel(s *chain.Stream) string {
	if s.TagPattern() == "" {
		return "stream " + s.Name()
	}
	return "stream " + s.Name() + "\n" + s.TagPattern()
}

func editKey(e *chain.Edit) string { return string(chain.EditID(e)) }

// holdingsOf is every Holding, the ones the pins make as well, by name: what a pin
// makes is a holding of its holder as much as one a document says.
func holdingsOf(g *chain.Graph) []*chain.Holding {
	var holdings []*chain.Holding
	for _, e := range g.Edges(chain.EdgeHolds) {
		holdings = append(holdings, e.Holding)
	}
	slices.SortFunc(holdings, func(a, c *chain.Holding) int { return strings.Compare(a.Name(), c.Name()) })
	return holdings
}

// editLabel names the edit and says what it does: the strategy that needs code,
// and how many steps run before and after the write.
func editLabel(e *chain.Edit) string {
	var does []string
	if e.Strategy() != "" {
		does = append(does, "strategy "+e.Strategy())
	}
	if n := len(e.PreEdit()); n > 0 {
		does = append(does, "pre "+steps(n))
	}
	if n := len(e.PostEdit()); n > 0 {
		does = append(does, "post "+steps(n))
	}
	if len(does) == 0 {
		return e.Name()
	}
	return e.Name() + "\n" + strings.Join(does, ", ")
}

func steps(n int) string {
	if n == 1 {
		return "1 step"
	}
	return fmt.Sprintf("%d steps", n)
}

func streamKey(s *chain.Stream) string     { return "stream:" + s.Name() }
func artifactKey(a *chain.Artifact) string { return "artifact:" + a.Name() }

// producerPath is where a holding's flow starts: the artifact, inside its stream
// and project, for a release, and the project for a commit or a branch.
func producerPath(h *chain.Holding) string {
	p := quote(h.Producer().Slug())
	if a := h.Artifact(); a != nil {
		return p + "." + quote(streamKey(a.Stream())) + "." + quote(artifactKey(a))
	}
	return p
}

func writeHolding(b *strings.Builder, g *chain.Graph, h *chain.Holding) {
	id := quote(string(chain.HoldingID(h)))
	label := h.Name() + "\n" + h.Kind()
	if h.Virtual() {
		label += ", pinned"
	}
	fmt.Fprintf(b, "%s: {class: [%s]; label: %s}\n", id, classHolding, quote(label))
	fmt.Fprintf(b, "%s -> %s\n", producerPath(h), id)
	var served []string
	for _, l := range g.Lines() {
		if h.AppliesTo(l.ID) {
			served = append(served, l.ID)
		}
	}
	edge := fmt.Sprintf("%s -> %s", id, quote(h.Holder().Slug()))
	if len(served) > 0 {
		edge += ": " + quote(strings.Join(served, ", "))
	}
	b.WriteString(edge + "\n")
}

// quote writes s as a D2 string, quoted so that the dots and slashes of a slug
// are not read as nesting or syntax.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}
