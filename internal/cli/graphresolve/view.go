package graphresolve

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
)

// view is what the options leave to draw: the projects drawn, the boundaries a
// drawn project touches, and the holdings and edges between them.
type view struct {
	projects []*chain.Project // drawn and boundaries, by slug
	boundary map[*chain.Project]bool
	holdings []*chain.Holding // those with a drawn end, by name
	follows  []chain.Edge     // of the edits that follow a holding drawn, by holding name
}

// newView chooses the projects the options draw (docs/specs/CONFIG_GRAPH.md,
// "Filtering") and what touches them.
func newView(g *chain.Graph, opts Options) (*view, error) {
	drawn, err := selectProjects(g.Projects(), opts)
	if err != nil {
		return nil, err
	}
	v := &view{boundary: map[*chain.Project]bool{}}
	inView := map[*chain.Project]bool{}
	for _, p := range drawn {
		inView[p] = true
	}
	for _, e := range g.Edges(chain.EdgeHolds) {
		h := e.Holding
		holder, producer := inView[h.Holder()], inView[h.Producer()]
		if !holder && !producer {
			continue
		}
		v.holdings = append(v.holdings, h)
		for _, p := range []*chain.Project{h.Holder(), h.Producer()} {
			if !inView[p] {
				v.boundary[p] = true
			}
		}
	}
	slices.SortFunc(v.holdings, func(a, c *chain.Holding) int { return strings.Compare(a.Name(), c.Name()) })
	v.projects = slices.Clone(drawn)
	for p := range v.boundary {
		v.projects = append(v.projects, p)
	}
	slices.SortFunc(v.projects, func(a, c *chain.Project) int { return strings.Compare(a.Slug(), c.Slug()) })

	kept := map[*chain.Holding]bool{}
	for _, h := range v.holdings {
		kept[h] = true
	}
	for _, e := range g.Edges(chain.EdgeFollows) {
		if kept[e.Holding] {
			v.follows = append(v.follows, e)
		}
	}
	slices.SortStableFunc(v.follows, func(a, c chain.Edge) int { return strings.Compare(a.Holding.Name(), c.Holding.Name()) })
	return v, nil
}

// drawnProjects are the projects drawn in full, as opposed to boundaries.
func (v *view) drawnProjects() []*chain.Project {
	var out []*chain.Project
	for _, p := range v.projects {
		if !v.boundary[p] {
			out = append(out, p)
		}
	}
	return out
}

// selectProjects is the projects the options choose, or an error when a pattern
// matches nothing or nothing is left.
func selectProjects(all []*chain.Project, opts Options) ([]*chain.Project, error) {
	var include, exclude []string
	for _, pattern := range opts.Projects {
		if rest, ok := strings.CutPrefix(pattern, "!"); ok {
			exclude = append(exclude, rest)
		} else {
			include = append(include, pattern)
		}
	}
	matched := func(pattern string) (func(*chain.Project) bool, error) {
		if _, err := path.Match(strings.ToLower(pattern), ""); err != nil {
			return nil, fmt.Errorf("project pattern %q: %w", pattern, err)
		}
		found := false
		for _, p := range all {
			found = found || matches(p, pattern)
		}
		if !found {
			return nil, fmt.Errorf("no project matches %q", pattern)
		}
		return func(p *chain.Project) bool { return matches(p, pattern) }, nil
	}
	var in, out []func(*chain.Project) bool
	for _, pattern := range include {
		m, err := matched(pattern)
		if err != nil {
			return nil, err
		}
		in = append(in, m)
	}
	for _, pattern := range exclude {
		m, err := matched(pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	var drawn []*chain.Project
	for _, p := range all {
		switch {
		case len(in) > 0 && !slices.ContainsFunc(in, func(m func(*chain.Project) bool) bool { return m(p) }):
			continue
		case len(in) == 0 && p.External() && !opts.External:
			continue
		case slices.ContainsFunc(out, func(m func(*chain.Project) bool) bool { return m(p) }):
			continue
		}
		drawn = append(drawn, p)
	}
	if len(drawn) == 0 {
		return nil, errors.New("no project is left to draw: the filters leave nothing")
	}
	return drawn, nil
}

// matches says whether a pattern matches the name, the slug or an alias of a
// project, whatever the case.
func matches(p *chain.Project, pattern string) bool {
	pattern = strings.ToLower(pattern)
	for _, name := range append([]string{p.Name(), p.Slug()}, p.Aliases()...) {
		if ok, _ := path.Match(pattern, strings.ToLower(name)); ok {
			return true
		}
	}
	return false
}
