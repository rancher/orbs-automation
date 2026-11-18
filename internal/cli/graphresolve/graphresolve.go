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
	"time"

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
	// KindER is the entity-relationship diagram of the config's schema, written as
	// Mermaid.
	KindER Kind = "er"
)

// Kinds lists the kinds Graph can draw, the default first.
var Kinds = []Kind{KindD2, KindMindmap, KindER}

// Options chooses which projects are drawn (docs/specs/CONFIG_GRAPH.md, "Filtering").
type Options struct {
	// External puts the external projects in.
	External bool
	// Projects are patterns on a project's name, slug and aliases; one that starts
	// with ! leaves out what it matches.
	Projects []string
	// ManagedBy keeps the projects whose managedBy is one of them.
	ManagedBy []string
	// Line draws one release line only.
	Line string
	// Now is the day the phase of a line is read at.
	Now time.Time
}

// Graph draws the central config found from s and the working directory cwd as
// a diagram of the kind asks for, of the projects the options choose.
func Graph(s clicfg.Settings, cwd string, kind Kind, opts Options) (string, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	return run(workspace.Open(s, cwd), s.Output, kind, opts)
}

// run draws the config as the output format and the kind ask for.
func run(ws Workspace, format string, kind Kind, opts Options) (string, error) {
	if format != clicfg.OutputText {
		return "", fmt.Errorf("output format %q is not supported by config resolve graph: it writes D2 or Mermaid", format)
	}
	switch kind {
	case KindD2:
		return graph(ws, opts)
	case KindMindmap:
		return mindmap(ws, opts)
	case KindER:
		return erDiagram(opts)
	}
	return "", fmt.Errorf("graph type %q is not supported by config resolve graph", kind)
}

// mindmap draws the config as a Mermaid mind map. A mind map is a tree and has
// no edges, so a Project is a branch of the root, and what joins two Projects is
// said on both of them: the artifact says where it reaches, and the holder says
// what it holds and from whom.
func mindmap(ws Workspace, opts Options) (string, error) {
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	v, err := newView(g, opts)
	if err != nil {
		return "", err
	}
	projects := v.drawnProjects()

	m := &mindmapWriter{}
	m.b.WriteString("mindmap\n")
	m.node(1, `root(("gero config"))`)
	for _, p := range projects {
		m.project(g, v, p)
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

func (m *mindmapWriter) project(g *chain.Graph, v *view, p *chain.Project) {
	holdings := v.holdings
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
	for _, st := range v.streams(p) {
		m.label(3, streamLabel(st))
		for _, a := range v.artifacts(st) {
			m.label(4, "artifact "+a.Name()+": "+a.Ref())
			for _, h := range holdings {
				if h.Artifact() == a {
					m.label(5, reaches(g, v, h))
				}
			}
		}
	}
	for _, h := range holdings {
		if h.Producer() == p && h.Artifact() == nil {
			m.label(3, reaches(g, v, h))
		}
	}
	if rows := v.rows(p); len(rows) > 0 {
		m.label(3, "lines")
		for _, r := range rows {
			text := r.text() + ": " + r.branchText()
			if r.series != "" {
				text += ", " + r.series
			}
			if phase := v.phase(r.line); phase != "" {
				text += " (" + phase + ")"
			}
			m.label(4, text)
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
func reaches(g *chain.Graph, v *view, h *chain.Holding) string {
	text := "to " + h.Holder().Name() + " via " + h.Name()
	var served []string
	for _, l := range g.Lines() {
		if v.line == "" && h.AppliesTo(l.ID) {
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
	classBoundary = "boundary"
	classLine     = "line"
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
  boundary: {style.fill: "#ffffff"; style.stroke-dash: 3; style.opacity: 0.5}
  line: {style.fill: "#ffffff"; style.stroke-width: 3}
  development: {style.stroke: "#4a90d9"}
  ga: {style.stroke: "#2e8b57"}
  eom: {style.stroke: "#d98e04"}
  eol: {style.stroke: "#b22222"}
}
`

func graph(ws Workspace, opts Options) (string, error) {
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	v, err := newView(g, opts)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(header)

	b.WriteString("\n")
	for _, p := range v.projects {
		if v.boundary[p] {
			fmt.Fprintf(&b, "%s: {class: [%s]; label: %s}\n", quote(p.Slug()), classBoundary, quote(p.Name()))
			continue
		}
		writeProject(&b, v, p)
	}

	// What a pin makes is a holding of its holder as well: a consumer says nothing of
	// lasso where it holds wrangler, and lasso still flows to it.
	b.WriteString("\n")
	for _, h := range v.holdings {
		writeHolding(&b, g, v, h)
	}
	edits := map[string]*chain.Edit{}
	for _, f := range v.follows {
		edits[f.Edit.Name()] = f.Edit
	}

	b.WriteString("\n")
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		fmt.Fprintf(&b, "%s: {class: [%s]; label: %s}\n", quote(editKey(edits[name])), classEdit, quote(editLabel(edits[name])))
	}
	for _, f := range v.follows { // a follows edge points at the Holding; the diagram flows from it
		fmt.Fprintf(&b, "%s -> %s: {style.stroke-dash: 4}\n", quote(string(f.To)), quote(string(f.From)))
	}
	return b.String(), nil
}

func writeProject(b *strings.Builder, v *view, p *chain.Project) {
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
	for _, st := range v.streams(p) {
		fmt.Fprintf(b, "  %s: {\n    class: [%s]\n    label: %s\n", quote(streamKey(st)), classStream, quote(streamLabel(st)))
		for _, a := range v.artifacts(st) {
			fmt.Fprintf(b, "    %s: {class: [%s]; label: %s}\n", quote(artifactKey(a)), classArtifact, quote(a.Name()+"\n"+a.Ref()))
		}
		b.WriteString("  }\n")
	}
	if rows := v.rows(p); len(rows) > 0 {
		b.WriteString("  \"lines\": {\n    label: \"release lines\"\n")
		for _, r := range rows {
			key, label := "line:"+r.line, r.text()+"\n"+r.branchText()
			if r.stage != "" {
				key += ":" + r.stage
			}
			if r.series != "" {
				label += "\n" + r.series
			}
			class := classLine
			if phase := v.phase(r.line); phase != "" {
				class += "; " + phase
			}
			fmt.Fprintf(b, "    %s: {class: [%s]; label: %s}\n", quote(key), class, quote(label))
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
func producerPath(v *view, h *chain.Holding) string {
	p := quote(h.Producer().Slug())
	if a := h.Artifact(); a != nil && !v.boundary[h.Producer()] {
		return p + "." + quote(streamKey(a.Stream())) + "." + quote(artifactKey(a))
	}
	return p
}

func writeHolding(b *strings.Builder, g *chain.Graph, v *view, h *chain.Holding) {
	id := quote(string(chain.HoldingID(h)))
	label := h.Name() + "\n" + h.Kind()
	if h.Virtual() {
		label += ", pinned"
	}
	fmt.Fprintf(b, "%s: {class: [%s]; label: %s}\n", id, classHolding, quote(label))
	fmt.Fprintf(b, "%s -> %s\n", producerPath(v, h), id)
	var served []string
	for _, l := range g.Lines() {
		if v.line == "" && h.AppliesTo(l.ID) {
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
