// Package graphresolve holds the logic of gero config resolve graph: it draws
// the whole central config as a D2 diagram, each Project a node and each holding
// a node between the Project that produces and the one that holds. It is
// framework-free: cmd/ owns the cobra wiring.
package graphresolve

import (
	"fmt"
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
	return graph(workspace.Open(s, cwd))
}

// The classes of a node. A Project has one of managed, shared and external, and
// hub as well when it is where releases end.
const (
	classManaged  = "managed"
	classShared   = "shared"
	classExternal = "external"
	classHub      = "hub"
	classHolding  = "holding"
)

const header = `direction: right

classes: {
  managed: {style.fill: "#d8f0e0"}
  shared: {style.fill: "#fdf0c8"}
  external: {style.fill: "#eeeeee"; style.stroke-dash: 3}
  hub: {style.stroke-width: 4}
  holding: {shape: hexagon; style.font-size: 12}
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
	for _, h := range holdings {
		writeHolding(&b, g, h)
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
	fmt.Fprintf(b, "%s: {class: [%s]; label: %s}\n", quote(p.Slug()), classes, quote(label))
}

func writeHolding(b *strings.Builder, g *chain.Graph, h *chain.Holding) {
	id := quote("holding:" + h.Name())
	label := h.Kind()
	if a := h.Artifact(); a != nil {
		label += "\n" + a.Ref()
	}
	fmt.Fprintf(b, "%s: {class: [%s]; label: %s}\n", id, classHolding, quote(label))
	fmt.Fprintf(b, "%s -> %s\n", quote(h.Producer().Slug()), id)
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
