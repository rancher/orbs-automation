// Package linesresolve holds the logic of gero lines resolve: the release-line map
// of the whole config, from the Rancher lines (newest first) through the project
// that is the end of the line for most things (rancher/rancher, the hub) to the
// projects that feed it and the ones it feeds. It reads the central config and the
// clock and never the network (gero repo status does). It is framework-free: cmd/
// owns the cobra wiring.
package linesresolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"io"
	"slices"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrNoHub is returned when no project can be taken as the hub of the map, and
// none was named.
var ErrNoHub = errors.New("no hub project")

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Chain is the central application config, joined: defaults applied and
	// validated.
	Chain() (*chain.Graph, error)
	// Bump builds the update engine, which routes the holdings.
	Bump() (*bump.Engine, error)
}

// Options is what the user asked for.
type Options struct {
	Hub    string   // the project the lines are connected through, by config name or slug; "" for the default
	Lines  []string // only these lines, such as v2.15 or 2.15; none for all
	Phases []string // only lines in these phases (development, ga, eom, eol); none for all
}

// Result is the map: one entry for each Rancher line, newest first.
type Result struct {
	Hub     string `json:"hub"`     // the hub's repository, owner/name
	HubName string `json:"hubName"` // its config name
	// HubIsFinalDestination says the hub is the project releases end in, so nothing
	// is shown as shipping on from it.
	HubIsFinalDestination bool   `json:"hubIsFinalDestination,omitempty"`
	Lines                 []Line `json:"lines"`
	// Gaps is every gap of every line, as the sentences each line carries, so a
	// reader can see at once whether the map has any.
	Gaps []string `json:"gaps,omitempty"`
}

// Line is one Rancher line and what is connected to it.
type Line struct {
	ID         string     `json:"id"`
	Phase      string     `json:"phase"`
	Dates      Dates      `json:"dates"`
	Kubernetes Kubernetes `json:"kubernetes"`
	// Hub is the hub's own branch and series for the line, nil when it has none.
	Hub *Node `json:"hub,omitempty"`
	// TakesFrom are the projects whose releases are shipped to the hub's branch for
	// the line, and ShipsTo the projects that the hub's releases for the line go to.
	TakesFrom []Link `json:"takesFrom,omitempty"`
	ShipsTo   []Link `json:"shipsTo,omitempty"`
	// Also are the projects that have the line but are not connected to the hub on
	// it directly, with their own connections, which is where a chain through
	// another project shows.
	Also []Also `json:"also,omitempty"`
	// Gaps are what does not connect: a project that ships to the hub but has no
	// such line, say.
	Gaps []string `json:"gaps,omitempty"`
}

// Dates are the lifecycle dates of a line, as YYYY-MM-DD, empty when not set.
type Dates struct {
	GA  string `json:"ga,omitempty"`
	EOM string `json:"eom,omitempty"`
	EOL string `json:"eol,omitempty"`
}

// Kubernetes is the supported Kubernetes minor range of a line.
type Kubernetes struct {
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
}

// Node is a project's own place on a line: its branch and the series of versions.
type Node struct {
	Project string `json:"project"` // the repository, owner/name
	Name    string `json:"name"`    // its config name
	Branch  string `json:"branch"`
	// IsDefaultBranch is set when the line has no branch of its own, so Branch is the
	// project's default branch.
	IsDefaultBranch bool   `json:"isDefaultBranch,omitempty"`
	Series          string `json:"series,omitempty"`
}

// Link is a connection to another project on a line: the project, which holdings
// make it, and what it holds of the line.
type Link struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	// Holdings are the holdings of the producer in the consumer that make the
	// connection, by name.
	Holdings []string `json:"holdings,omitempty"`
	// Series and Branch are the other project's own series and branch for the line,
	// when it ships (a producer) and has them.
	Series string `json:"series,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Universal says the producer has no lines of its own: its releases go to every
	// release branch of the consumer, whatever the line.
	Universal bool `json:"universal,omitempty"`
	// Branches are the consumer's branches a release goes to, for a link to a
	// consumer.
	Branches []string `json:"branches,omitempty"`
}

// Also is a project on a line that is not connected to the hub on it directly.
type Also struct {
	Node
	TakesFrom []Link `json:"takesFrom,omitempty"`
	ShipsTo   []Link `json:"shipsTo,omitempty"`
}

// Resolve builds the map from the config and the engine, at the time now. The
// hub is the project named in opts, else the one marked finalDestination, and
// every project connected to it on a line is connected by the
// routes of the engine, which are the ones gero ship ships down, so the map and a
// run agree. One hop from the hub is followed; a project that has a line but no
// direct link to the hub on it is listed with its own connections.
func Resolve(g *chain.Graph, engine *bump.Engine, opts Options, now time.Time) (Result, error) {
	if len(g.Lines()) == 0 {
		return Result{}, errors.New("the config has no RancherLines document, so there are no lines to map")
	}
	hub, err := hubOf(g, opts.Hub)
	if err != nil {
		return Result{}, err
	}
	doc, err := selectLines(g.Lines(), opts, now)
	if err != nil {
		return Result{}, err
	}
	m := mapper{g: g, engine: engine, hub: hub}
	res := Result{Hub: hub.Slug(), HubName: hub.Name(), HubIsFinalDestination: hub.FinalDestination()}
	for _, l := range doc {
		line := m.line(l, now)
		res.Gaps = append(res.Gaps, line.Gaps...)
		res.Lines = append(res.Lines, line)
	}
	return res, nil
}

// hubOf finds the project the lines are connected through: the one named, else the
// one project marked finalDestination, and no project by its name.
func hubOf(g *chain.Graph, name string) (*chain.Project, error) {
	if name != "" {
		if p := g.Project(name); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("%w: %q is not a project in the config", ErrNoHub, name)
	}
	for _, p := range g.Projects() {
		if p.FinalDestination() {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: no project is marked finalDestination: true, so name one with --hub", ErrNoHub)
}

var phases = []string{lines.PhaseDevelopment, lines.PhaseGA, lines.PhaseEOM, lines.PhaseEOL}

// selectLines is the lines of the document that the options keep, newest first.
func selectLines(all []lines.Line, opts Options, now time.Time) ([]lines.Line, error) {
	sorted := slices.Clone(all)
	slices.SortStableFunc(sorted, func(a, b lines.Line) int { return -lines.Compare(a.ID, b.ID) })
	for _, p := range opts.Phases {
		if !slices.Contains(phases, p) {
			return nil, fmt.Errorf("%q is not a phase: it is %s", p, strings.Join(phases, ", "))
		}
	}
	var want []string
	for _, id := range opts.Lines {
		id = lines.Normalize(id)
		if !slices.ContainsFunc(sorted, func(l lines.Line) bool { return strings.EqualFold(l.ID, id) }) {
			ids := make([]string, len(sorted))
			for i, l := range sorted {
				ids[i] = l.ID
			}
			return nil, fmt.Errorf("%s is not a Rancher line: they are %s", id, strings.Join(ids, ", "))
		}
		want = append(want, strings.ToLower(id))
	}
	var out []lines.Line
	for _, l := range sorted {
		if len(want) > 0 && !slices.Contains(want, strings.ToLower(l.ID)) {
			continue
		}
		if len(opts.Phases) > 0 && !slices.Contains(opts.Phases, l.PhaseAt(now)) {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

// mapper holds what is the same for every line.
type mapper struct {
	g      *chain.Graph
	engine *bump.Engine
	hub    *chain.Project
}

// line connects one Rancher line to the hub and to the rest.
func (m mapper) line(l lines.Line, now time.Time) Line {
	out := Line{
		ID: l.ID, Phase: l.PhaseAt(now),
		Dates:      Dates{GA: date(l.Dates.GA), EOM: date(l.Dates.EOM), EOL: date(l.Dates.EOL)},
		Kubernetes: Kubernetes{Min: l.Kubernetes.Min, Max: l.Kubernetes.Max},
	}
	hubNode, hubHas := nodeOf(m.hub, l.ID)
	if hubHas {
		out.Hub = &hubNode
	}
	linked := map[string]bool{m.hub.Name(): true}
	for _, p := range m.g.Projects() {
		if p.Name() == m.hub.Name() {
			continue
		}
		for _, r := range m.engine.RoutesFrom(p) {
			if r.Consumer.Name() != m.hub.Name() {
				continue
			}
			link, gap, ok := m.feedsHub(p, r, l.ID, hubNode, hubHas)
			if gap != "" {
				out.Gaps = append(out.Gaps, gap)
			}
			if ok {
				out.TakesFrom = append(out.TakesFrom, link)
				linked[p.Name()] = true
			}
		}
	}
	// Nothing ships on from the final destination, whatever holdings it is the
	// producer of, so they are not shown as that, and the projects that hold
	// something of it are then just on the line.
	if hubHas && !m.hub.FinalDestination() {
		for _, r := range m.engine.RoutesFrom(m.hub) {
			if link, ok := shipsTo(r, l.ID); ok {
				out.ShipsTo = append(out.ShipsTo, link)
				linked[r.Consumer.Name()] = true
			}
		}
	}
	for _, p := range m.g.Projects() {
		node, has := nodeOf(p, l.ID)
		if !has || linked[p.Name()] {
			continue
		}
		also := Also{Node: node}
		for _, r := range m.engine.RoutesInto(p) {
			if link, ok := producerLink(r, l.ID); ok {
				also.TakesFrom = append(also.TakesFrom, link)
			}
		}
		for _, r := range m.engine.RoutesFrom(p) {
			if link, ok := shipsTo(r, l.ID); ok {
				also.ShipsTo = append(also.ShipsTo, link)
			}
		}
		out.Also = append(out.Also, also)
	}
	return out
}

// universal says whether the releases of the route's producer go to every release
// branch of the consumer: the streams of what it holds have no lines of their own.
func universal(r bump.Route) bool {
	for _, h := range r.Holdings {
		if h.Artifact().Stream().Universal() {
			return true
		}
	}
	return false
}

// holdingsOn are the names of the holdings of a route that cover a line: a route
// that holds several streams of a producer has a holding for each, and each is for
// some lines only.
func holdingsOn(r bump.Route, line string) []string {
	var names []string
	for _, h := range r.Holdings {
		if h.Doc.Spec.Holds.Lines.Contains(line) {
			names = append(names, h.Name())
		}
	}
	return names
}

// holdingNames are the names of the holdings of a route.
func holdingNames(r bump.Route) []string {
	names := make([]string, len(r.Holdings))
	for i, h := range r.Holdings {
		names[i] = h.Name()
	}
	return names
}

// branchesOn are the consumer's branches a release of the producer goes to for a
// line: every release branch for a universal producer, else the branches of the
// line.
func branchesOn(r bump.Route, line string) ([]string, error) {
	var targets []bump.Target
	var err error
	if universal(r) {
		targets, err = r.TargetsFor("")
	} else {
		targets, err = r.TargetsOn(line)
	}
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, t := range targets {
		if !slices.Contains(branches, t.Branch) {
			branches = append(branches, t.Branch)
		}
	}
	return branches, nil
}

// tagLines says the project finds its lines in its tags: one branch for them all,
// and streams that list the lines.
func tagLines(p *chain.Project) bool {
	if p.Layout() != chain.LayoutSingle {
		return false
	}
	return slices.ContainsFunc(p.Streams(), func(s *chain.Stream) bool { return len(s.Lines()) > 0 })
}

// feedsHub says whether producer p ships to the hub's branch for the line by route
// r, and when it should but cannot, why (a gap). A producer with no lines of its own
// ships every release to every release branch of the hub, so it feeds the line the
// hub's branch is one of them. One that follows lines feeds the line it has, and has
// a gap for one it lacks, unless its lines are in its tags: it lists the lines it is
// used for, and only those are judged.
func (m mapper) feedsHub(p *chain.Project, r bump.Route, line string, hubNode Node, hubHas bool) (Link, string, bool) {
	uni := universal(r)
	pl, pHas := nodeOf(p, line)
	switch {
	case !hubHas && pHas && !uni:
		return Link{}, fmt.Sprintf("%s has the line %s, which %s does not have, so nothing of it ships there", p.Name(), line, m.hub.Name()), false
	case !hubHas:
		return Link{}, "", false
	case uni:
		branches, err := branchesOn(r, "")
		if err != nil || !slices.Contains(branches, hubNode.Branch) {
			return Link{}, "", false
		}
		return Link{Project: p.Slug(), Name: p.Name(), Holdings: holdingNames(r), Universal: true}, "", true
	case !pHas && tagLines(p):
		// Its lines are the ones it is used for, and it lists those, so a line it does
		// not list is not one it is meant to feed.
		return Link{}, "", false
	case !pHas:
		return Link{}, fmt.Sprintf("%s has no line %s, so nothing of it ships to %s %s", p.Name(), line, m.hub.Name(), line), false
	}
	branches, err := branchesOn(r, line)
	if err != nil || !slices.Contains(branches, hubNode.Branch) {
		return Link{}, fmt.Sprintf("%s %s does not reach %s %s (%v)", p.Name(), line, m.hub.Name(), line, err), false
	}
	return Link{Project: p.Slug(), Name: p.Name(), Holdings: holdingNames(r), Series: pl.Series, Branch: pl.Branch}, "", true
}

// producerLink is the link of a producer on a line, as the project it ships to has
// it: universal, or its series and branch for the line. It is false for a producer
// that follows lines and has none.
func producerLink(r bump.Route, line string) (Link, bool) {
	p := r.Producer
	held := holdingsOn(r, line)
	if len(held) == 0 {
		return Link{}, false // none of its holdings is for this line
	}
	if universal(r) {
		return Link{Project: p.Slug(), Name: p.Name(), Holdings: held, Universal: true}, true
	}
	pl, ok := nodeOf(p, line)
	if !ok {
		return Link{}, false
	}
	return Link{Project: p.Slug(), Name: p.Name(), Holdings: held, Series: pl.Series, Branch: pl.Branch}, true
}

// shipsTo is the link to the consumer of route r that a release of its producer for
// the line goes to, with the consumer's branches. It is false when none can be worked
// out, such as a consumer that does not have the line.
func shipsTo(r bump.Route, line string) (Link, bool) {
	held := holdingsOn(r, line)
	if len(held) == 0 {
		return Link{}, false
	}
	branches, err := branchesOn(r, line)
	if err != nil {
		return Link{}, false
	}
	return Link{Project: r.Consumer.Slug(), Name: r.Consumer.Name(), Holdings: held, Branches: branches}, true
}

// nodeOf is the project's own place on a line: false when it has no such line. The
// series is only set when a stream of the project writes one for the line. A
// project that keeps each line on a branch has the lines it lists; one that finds
// its lines in its tags has the lines its streams list, all on its default branch.
func nodeOf(p *chain.Project, line string) (Node, bool) {
	rows := p.ReleaseTargetsOf(line)
	if len(rows) == 0 {
		return Node{}, false
	}
	node := Node{Project: p.Slug(), Name: p.Name(), Series: rows[0].Series}
	// A line that has a branch for each stage has no one branch to name.
	if len(rows) == 1 && rows[0].Stage == "" {
		node.Branch, node.IsDefaultBranch = rows[0].Branch, rows[0].IsDefault
	}
	return node, true
}

func date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// Run writes the map to w in the format the settings ask for.
func Run(_ context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	return run(w, workspace.Open(s, cwd), s.Output, opts, time.Now(), style.New(s.Terminal))
}

// run is Run over its dependencies. An engine that is only partly usable (an
// holding or an edit that is not usable) still routes the holdings that are, so it is used and
// the problem is left to gero doctor and gero config validate.
func run(w io.Writer, ws Workspace, format string, opts Options, now time.Time, view *style.Renderer) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by lines resolve: use text or json", format)
	}
	g, err := ws.Chain()
	if err != nil {
		return err
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return fmt.Errorf("the update engine cannot be built: %w", engineErr)
	}
	res, err := Resolve(g, engine, opts, now)
	if err != nil {
		return err
	}
	if format == clicfg.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	_, err = io.WriteString(w, renderText(res, view))
	return err
}

// renderText is the map as text: a block for each line, newest first, with the
// hub's branch, what it takes from and ships to, the projects also on the line
// and the gaps, and a count of the gaps at the end.
func renderText(r Result, view *style.Renderer) string {
	var b strings.Builder
	b.WriteString(view.Style(style.Heading, "Release lines, through "+r.Hub) + "\n")
	for _, l := range r.Lines {
		b.WriteString("\n" + lineHeader(view, l) + "\n")
		if l.Hub == nil {
			fmt.Fprintf(&b, "  %s  %s\n", r.Hub, view.Style(style.Warn, "(no such line)"))
		} else {
			fmt.Fprintf(&b, "  %s  %s\n", l.Hub.Project, branchText(*l.Hub))
		}
		b.WriteString("    " + view.Style(style.Dim, "takes from") + "\n")
		writeLinks(&b, view, "      ", l.TakesFrom, takesRow)
		if !r.HubIsFinalDestination {
			b.WriteString("    " + view.Style(style.Dim, "ships to") + "\n")
			writeLinks(&b, view, "      ", l.ShipsTo, shipsRow)
		}
		if len(l.Also) > 0 {
			b.WriteString("    " + view.Style(style.Dim, "also on this line") + "\n")
			for _, a := range l.Also {
				fmt.Fprintf(&b, "      %s  %s\n", a.Name, branchText(a.Node))
				for _, in := range a.TakesFrom {
					b.WriteString("        " + view.Style(style.Dim, fmt.Sprintf("takes from %s (%s) via %s", in.Name, seriesText(in), via(in))) + "\n")
				}
				for _, out := range a.ShipsTo {
					b.WriteString("        " + view.Style(style.Dim, fmt.Sprintf("ships to %s (%s) via %s", out.Name, strings.Join(out.Branches, ", "), via(out))) + "\n")
				}
			}
		}
		for _, gap := range l.Gaps {
			b.WriteString("  " + view.Style(style.Warn, "! "+gap) + "\n")
		}
	}
	if len(r.Gaps) == 0 {
		b.WriteString("\n" + view.Style(style.OK, "No gaps.") + "\n")
	} else {
		b.WriteString("\n" + view.Style(style.Warn, fmt.Sprintf("Gaps: %d", len(r.Gaps))) + "\n")
	}
	return b.String()
}

// phaseRole is the look of where a line is in its life.
func phaseRole(phase string) style.Role {
	switch phase {
	case lines.PhaseGA:
		return style.OK
	case lines.PhaseEOM:
		return style.Warn
	case lines.PhaseEOL:
		return style.Error
	}
	return style.Dim
}

// lineHeader is a line's id, its phase, its dates and its Kubernetes range. The
// id is a heading, the phase takes the look of where the line is, and the rest
// is secondary text.
func lineHeader(view *style.Renderer, l Line) string {
	parts := []string{view.Style(style.Heading, l.ID), view.Style(phaseRole(l.Phase), l.Phase)}
	var dates []string
	for _, d := range []struct{ label, value string }{{"GA", l.Dates.GA}, {"EOM", l.Dates.EOM}, {"EOL", l.Dates.EOL}} {
		if d.value != "" {
			dates = append(dates, d.label+" "+d.value)
		}
	}
	if len(dates) > 0 {
		parts = append(parts, view.Style(style.Dim, strings.Join(dates, ", ")))
	}
	if l.Kubernetes.Min != "" || l.Kubernetes.Max != "" {
		parts = append(parts, view.Style(style.Dim, "Kubernetes "+l.Kubernetes.Min+"-"+l.Kubernetes.Max))
	}
	return strings.Join(parts, "  ")
}

func branchText(n Node) string {
	text := n.Branch
	if n.IsDefaultBranch {
		text += " (default branch)"
	}
	if n.Series != "" {
		text += "  series " + n.Series
	}
	return text
}

// seriesText is what a producer holds of a line: its series, or any release.
func seriesText(l Link) string {
	if l.Universal {
		return "any release"
	}
	return l.Series
}

func takesRow(view *style.Renderer, l Link) []string {
	branch := l.Branch
	if l.Universal {
		branch = ""
	}
	return []string{l.Name, seriesText(l), branch, view.Style(style.Dim, via(l))}
}

// via is the holdings that make a link.
func via(l Link) string { return strings.Join(l.Holdings, ", ") }

func shipsRow(view *style.Renderer, l Link) []string {
	return []string{l.Name, strings.Join(l.Branches, ", "), view.Style(style.Dim, via(l))}
}

// writeLinks writes a group of links as aligned rows, or a dash for none.
func writeLinks(b *strings.Builder, view *style.Renderer, indent string, links []Link, row func(*style.Renderer, Link) []string) {
	if len(links) == 0 {
		b.WriteString(indent + view.Style(style.Dim, "-") + "\n")
		return
	}
	table := make([][]string, len(links))
	for i, l := range links {
		cells := row(view, l)
		cells[0] = indent + cells[0]
		table[i] = cells
	}
	b.WriteString(ui.Align(table, view.Width))
}
