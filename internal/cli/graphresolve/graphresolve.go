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
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Graph needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Chain() (*chain.Graph, error)
}

// Graph draws the central config found from s and the working directory cwd as
// D2 source.
func Graph(s clicfg.Settings, cwd string) (string, error) {
	return run(workspace.Open(s, cwd), s.Output)
}

// run draws the config as the output format asks for.
func run(ws Workspace, format string) (string, error) {
	if format != clicfg.OutputText {
		return "", fmt.Errorf("output format %q is not supported by config resolve graph: it writes D2", format)
	}
	return graph(ws)
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

	holdings := slices.Clone(g.Holdings())
	slices.SortFunc(holdings, func(a, c *chain.Holding) int { return strings.Compare(a.Name(), c.Name()) })
	b.WriteString("\n")
	edits := map[string]*chain.Edit{}
	for _, h := range holdings {
		writeHolding(&b, g, h)
		for _, e := range h.Edits() {
			edits[e.Name()] = e
		}
	}

	b.WriteString("\n")
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		fmt.Fprintf(&b, "%s: {class: [%s]; label: %s}\n", quote(editKey(edits[name])), classEdit, quote(editLabel(edits[name])))
	}
	for _, h := range holdings {
		for _, e := range h.Edits() {
			fmt.Fprintf(&b, "%s -> %s: {style.stroke-dash: 4}\n", quote("holding:"+h.Name()), quote(editKey(e)))
		}
	}
	return b.String(), nil
}

func writeProject(b *strings.Builder, p *chain.Project) {
	class, label := classManaged, p.Name()
	switch {
	case p.Doc.Spec.Repo.External:
		class = classExternal
	case p.Shared():
		class = classShared
	case p.ManagedBy() != "":
		label += "\nmanaged by " + p.ManagedBy()
	}
	classes := class
	if p.Doc.Spec.FinalDestination {
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

func editKey(e *chain.Edit) string { return "edit:" + e.Name() }

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
	id := quote("holding:" + h.Name())
	label := h.Name() + "\n" + h.Kind()
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
