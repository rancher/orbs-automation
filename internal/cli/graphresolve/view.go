package graphresolve

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/chain"
)

// view is what the options leave to draw: the projects drawn, the boundaries a
// drawn project touches, and the holdings and edges between them.
type view struct {
	projects []*chain.Project // drawn and boundaries, by slug
	boundary map[*chain.Project]bool
	holdings []*chain.Holding // those with a drawn end, by name
	follows  []chain.Edge     // of the edits that follow a holding drawn, by holding name
	line     string           // the one line drawn, as the lines document writes it; empty for all
	phase    func(line string) string
}

// newView chooses the projects the options draw (docs/specs/CONFIG_GRAPH.md,
// "Filtering") and what touches them.
func newView(g *chain.Graph, opts Options) (*view, error) {
	drawn, err := selectProjects(g.Projects(), opts)
	if err != nil {
		return nil, err
	}
	v := &view{boundary: map[*chain.Project]bool{}, phase: func(line string) string {
		if l, ok := g.Line(line); ok {
			return l.PhaseAt(opts.Now)
		}
		return ""
	}}
	if opts.Line != "" {
		l, ok := g.Line(opts.Line)
		if !ok {
			var known []string
			for _, l := range g.Lines() {
				known = append(known, l.ID)
			}
			return nil, fmt.Errorf("no release line %q: the config has %s", opts.Line, strings.Join(known, ", "))
		}
		v.line = l.ID
	}
	forLine := func(h *chain.Holding) bool { return v.line == "" || h.AppliesTo(v.line) }
	if v.line != "" {
		touched := map[*chain.Project]bool{}
		for _, e := range g.Edges(chain.EdgeHolds) {
			if forLine(e.Holding) {
				touched[e.Holding.Holder()], touched[e.Holding.Producer()] = true, true
			}
		}
		drawn = slices.DeleteFunc(slices.Clone(drawn), func(p *chain.Project) bool {
			return !touched[p] && len(v.rows(p)) == 0
		})
		if len(drawn) == 0 {
			return nil, fmt.Errorf("nothing is left to draw for line %s", v.line)
		}
	}
	inView := map[*chain.Project]bool{}
	for _, p := range drawn {
		inView[p] = true
	}
	for _, e := range g.Edges(chain.EdgeHolds) {
		h := e.Holding
		if !forLine(h) {
			continue
		}
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
	for _, team := range opts.ManagedBy {
		if !slices.ContainsFunc(all, func(p *chain.Project) bool { return managedBy(p, team) }) {
			return nil, fmt.Errorf("no project is managed by %q", team)
		}
	}
	var drawn []*chain.Project
	for _, p := range all {
		switch {
		case len(opts.ManagedBy) > 0 && !slices.ContainsFunc(opts.ManagedBy, func(team string) bool { return managedBy(p, team) }):
			continue
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

// managedBy says whether the project's managedBy is the team, whatever the case.
func managedBy(p *chain.Project, team string) bool {
	return slices.ContainsFunc(p.ManagedBy(), func(id string) bool { return strings.EqualFold(id, team) })
}

// row is one line of a project: where it is branched, and what it releases.
type row struct {
	line, stage, branch, series string
	pending                     bool
}

// rows are the release lines of a project to draw, newest line first and a line's
// stages in the order of the project's, only the one line when the view is of one.
func (v *view) rows(p *chain.Project) []row {
	var out []row
	for _, t := range p.ReleaseTargets() {
		if t.Line == "" || (v.line != "" && lines.Normalize(t.Line) != lines.Normalize(v.line)) {
			continue
		}
		out = append(out, row{line: t.Line, stage: t.Stage, branch: t.Branch, series: t.Series, pending: t.Branch == ""})
	}
	slices.SortStableFunc(out, func(a, c row) int { return lines.Compare(c.line, a.line) })
	return out
}

// text is the row as one line of words: the line and stage, its branch and series.
func (r row) text() string {
	text := r.line
	if r.stage != "" {
		text += " " + r.stage
	}
	return text
}

// branchText is where the row's line is branched.
func (r row) branchText() string {
	if r.pending {
		return "no branch yet"
	}
	return r.branch
}

// streams and artifacts are the ones of a project that are for the line, or all of
// them when the view is of every line.
func (v *view) streams(p *chain.Project) []*chain.Stream {
	return slices.DeleteFunc(slices.Clone(p.Streams()), func(st *chain.Stream) bool {
		return v.line != "" && !st.Serves(v.line)
	})
}

func (v *view) artifacts(st *chain.Stream) []*chain.Artifact {
	return slices.DeleteFunc(slices.Clone(st.Artifacts()), func(a *chain.Artifact) bool {
		return v.line != "" && !a.AppliesTo(v.line)
	})
}
