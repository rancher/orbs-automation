// Package linesresolve holds the logic of gero lines resolve: the release-line map
// of the whole config, from the Rancher lines (newest first) through the project
// that is the end of the line for most things (rancher/rancher, the hub) to the
// projects that feed it and the ones it feeds. It reads the central config and the
// clock and never the network (gero repo status does). It is framework-free: cmd/
// owns the cobra wiring.
package linesresolve

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrNoHub is returned when no project can be taken as the hub of the map, and
// none was named.
var ErrNoHub = errors.New("no hub project")

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Config is the central application config, with defaults applied and
	// validated.
	Config() (config.Config, error)
	// Updates builds the engine of the update methods, which routes the edges.
	Updates() (*updatedispatcher.Engine, error)
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

// Link is a connection to another project on a line: the project, how the update is
// made, and what it holds of the line.
type Link struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	Method  string `json:"method"`
	// Params are what the edge passes to the method, such as the key it updates.
	Params map[string]string `json:"params,omitempty"`
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
func Resolve(cfg config.Config, engine *updatedispatcher.Engine, opts Options, now time.Time) (Result, error) {
	if cfg.Lines == nil {
		return Result{}, errors.New("the config has no RancherLines document, so there are no lines to map")
	}
	hub, err := hubOf(cfg, opts.Hub)
	if err != nil {
		return Result{}, err
	}
	doc, err := selectLines(cfg.Lines.Lines, opts, now)
	if err != nil {
		return Result{}, err
	}
	m := mapper{cfg: cfg, engine: engine, hub: hub}
	res := Result{Hub: hub.Repo.Name, HubName: hub.Name, HubIsFinalDestination: hub.FinalDestination}
	for _, l := range doc {
		line := m.line(l, now)
		res.Gaps = append(res.Gaps, line.Gaps...)
		res.Lines = append(res.Lines, line)
	}
	return res, nil
}

// hubOf finds the project the lines are connected through: the one named, else the
// one project marked finalDestination, and no project by its name.
func hubOf(cfg config.Config, name string) (config.Project, error) {
	if name != "" {
		for _, p := range cfg.Projects {
			if p.Name == name || strings.EqualFold(p.Repo.Name, name) {
				return p, nil
			}
		}
		return config.Project{}, fmt.Errorf("%w: %q is not a project in the config", ErrNoHub, name)
	}
	for _, p := range cfg.Projects {
		if p.FinalDestination {
			return p, nil
		}
	}
	return config.Project{}, fmt.Errorf("%w: no project is marked finalDestination: true, so name one with --hub", ErrNoHub)
}

var phases = []string{config.PhaseDevelopment, config.PhaseGA, config.PhaseEOM, config.PhaseEOL}

// selectLines is the lines of the document that the options keep, newest first.
func selectLines(all []config.Line, opts Options, now time.Time) ([]config.Line, error) {
	lines := slices.Clone(all)
	slices.SortStableFunc(lines, func(a, b config.Line) int { return -compareLineIDs(a.ID, b.ID) })
	for _, p := range opts.Phases {
		if !slices.Contains(phases, p) {
			return nil, fmt.Errorf("%q is not a phase: it is %s", p, strings.Join(phases, ", "))
		}
	}
	var want []string
	for _, id := range opts.Lines {
		id = withV(id)
		if !slices.ContainsFunc(lines, func(l config.Line) bool { return strings.EqualFold(l.ID, id) }) {
			ids := make([]string, len(lines))
			for i, l := range lines {
				ids[i] = l.ID
			}
			return nil, fmt.Errorf("%s is not a Rancher line: they are %s", id, strings.Join(ids, ", "))
		}
		want = append(want, strings.ToLower(id))
	}
	var out []config.Line
	for _, l := range lines {
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

// compareLineIDs orders line ids such as v2.9 and v2.16 by their numbers.
func compareLineIDs(a, b string) int {
	an, bn := lineNumbers(a), lineNumbers(b)
	for i := 0; i < len(an) && i < len(bn); i++ {
		if c := cmp.Compare(an[i], bn[i]); c != 0 {
			return c
		}
	}
	return cmp.Or(cmp.Compare(len(an), len(bn)), strings.Compare(a, b))
}

func lineNumbers(id string) []int {
	var out []int
	for _, part := range strings.Split(strings.TrimPrefix(strings.ToLower(id), "v"), ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return out
		}
		out = append(out, n)
	}
	return out
}

func withV(id string) string {
	if !strings.HasPrefix(strings.ToLower(id), "v") {
		return "v" + id
	}
	return id
}

// mapper holds what is the same for every line.
type mapper struct {
	cfg    config.Config
	engine *updatedispatcher.Engine
	hub    config.Project
}

// line connects one Rancher line to the hub and to the rest.
func (m mapper) line(l config.Line, now time.Time) Line {
	out := Line{
		ID: l.ID, Phase: l.PhaseAt(now),
		Dates:      Dates{GA: date(l.Dates.GA), EOM: date(l.Dates.EOM), EOL: date(l.Dates.EOL)},
		Kubernetes: Kubernetes{Min: l.Kubernetes.Min, Max: l.Kubernetes.Max},
	}
	hubNode, hubHas := nodeOf(m.hub, l.ID)
	if hubHas {
		out.Hub = &hubNode
	}
	linked := map[string]bool{m.hub.Name: true}
	for _, p := range m.cfg.Projects {
		if p.Name == m.hub.Name {
			continue
		}
		for _, r := range m.engine.RoutesFrom(p) {
			if r.Consumer.Name != m.hub.Name {
				continue
			}
			link, gap, ok := m.feedsHub(p, r, l.ID, hubNode, hubHas)
			if gap != "" {
				out.Gaps = append(out.Gaps, gap)
			}
			if ok {
				out.TakesFrom = append(out.TakesFrom, link)
				linked[p.Name] = true
			}
		}
	}
	// Nothing ships on from the final destination, whatever edges it has, so they
	// are not shown as that, and the projects it has edges to are then just on the
	// line.
	if hubHas && !m.hub.FinalDestination {
		for _, r := range m.engine.RoutesFrom(m.hub) {
			if link, ok := shipsTo(m.hub, r, l.ID); ok {
				out.ShipsTo = append(out.ShipsTo, link)
				linked[r.Consumer.Name] = true
			}
		}
	}
	for _, p := range m.cfg.Projects {
		node, has := nodeOf(p, l.ID)
		if !has || linked[p.Name] {
			continue
		}
		also := Also{Node: node}
		for _, in := range m.engine.RoutesInto(p) {
			if link, ok := producerLink(in.Producer, in.Route, l.ID); ok {
				also.TakesFrom = append(also.TakesFrom, link)
			}
		}
		for _, r := range m.engine.RoutesFrom(p) {
			if link, ok := shipsTo(p, r, l.ID); ok {
				also.ShipsTo = append(also.ShipsTo, link)
			}
		}
		out.Also = append(out.Also, also)
	}
	return out
}

// feedsHub says whether producer p ships to the hub's branch for the line by route
// r, and when it should but cannot, why (a gap). A producer with no lines of its own
// ships every release to every release branch of the hub, so it feeds the line the
// hub's branch is one of them. One that follows lines feeds the line it has, and has
// a gap for one it lacks, unless its lines are in its tags (perLineTag): it lists
// the lines it is used for, and only those are judged.
func (m mapper) feedsHub(p config.Project, r updatedispatcher.Route, line string, hubNode Node, hubHas bool) (Link, string, bool) {
	universal := r.Edge.AcceptLines(p) == config.AcceptLinesUniversal
	pl, pHas := nodeOf(p, line)
	switch {
	case !hubHas && pHas && !universal:
		return Link{}, fmt.Sprintf("%s has the line %s, which %s does not have, so nothing of it ships there", p.Name, line, m.hub.Name), false
	case !hubHas:
		return Link{}, "", false
	case universal:
		branches, err := r.BranchesFor(p, "")
		if err != nil || !slices.Contains(branches, hubNode.Branch) {
			return Link{}, "", false
		}
		return Link{Project: p.Repo.Name, Name: p.Name, Method: r.Method.Name, Params: r.Edge.Params, Universal: true}, "", true
	case !pHas && p.Release.Branching == config.BranchingPerLineTag:
		// Its lines are the ones it is used for, and it lists those, so a line it does
		// not list is not one it is meant to feed.
		return Link{}, "", false
	case !pHas:
		return Link{}, fmt.Sprintf("%s has no line %s, so nothing of it ships to %s %s", p.Name, line, m.hub.Name, line), false
	}
	branches, err := r.BranchesFor(p, line)
	if err != nil || !slices.Contains(branches, hubNode.Branch) {
		return Link{}, fmt.Sprintf("%s %s does not reach %s %s (%v)", p.Name, line, m.hub.Name, line, err), false
	}
	return Link{Project: p.Repo.Name, Name: p.Name, Method: r.Method.Name, Params: r.Edge.Params, Series: pl.Series, Branch: pl.Branch}, "", true
}

// producerLink is the link of a producer on a line, as the project it ships to has
// it: universal, or its series and branch for the line. It is false for a producer
// that follows lines and has none.
func producerLink(p config.Project, r updatedispatcher.Route, line string) (Link, bool) {
	if r.Edge.AcceptLines(p) == config.AcceptLinesUniversal {
		return Link{Project: p.Repo.Name, Name: p.Name, Method: r.Method.Name, Params: r.Edge.Params, Universal: true}, true
	}
	pl, ok := nodeOf(p, line)
	if !ok {
		return Link{}, false
	}
	return Link{Project: p.Repo.Name, Name: p.Name, Method: r.Method.Name, Params: r.Edge.Params, Series: pl.Series, Branch: pl.Branch}, true
}

// shipsTo is the link to the consumer of route r that a release of p for the line
// goes to, with the consumer's branches. It is false when none can be worked out,
// such as a consumer that does not have the line.
func shipsTo(p config.Project, r updatedispatcher.Route, line string) (Link, bool) {
	arg := line
	if r.Edge.AcceptLines(p) == config.AcceptLinesUniversal {
		arg = ""
	}
	branches, err := r.BranchesFor(p, arg)
	if err != nil {
		return Link{}, false
	}
	return Link{Project: r.Consumer.Repo.Name, Name: r.Consumer.Name, Method: r.Method.Name, Params: r.Edge.Params, Branches: branches}, true
}

// nodeOf is the project's own place on a line: false when it has no such line. The
// series is only set when the project writes one of its own for the line.
func nodeOf(p config.Project, line string) (Node, bool) {
	for _, l := range p.Release.Lines {
		if !strings.EqualFold(l.Line, line) {
			continue
		}
		branch, _ := p.BranchOfLine(l.Line)
		n := Node{Project: p.Repo.Name, Name: p.Name, Branch: branch, IsDefaultBranch: branch == p.Repo.DefaultBranch && (l.Branch == "" || p.Release.Branching == config.BranchingSingle)}
		if l.Version != "" {
			n.Series = l.Version
		}
		return n, true
	}
	return Node{}, false
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
// update method that is not) still routes the edges that are, so it is used and
// the problem is left to gero doctor and gero config validate.
func run(w io.Writer, ws Workspace, format string, opts Options, now time.Time, view *style.Renderer) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by lines resolve: use text or json", format)
	}
	cfg, err := ws.Config()
	if err != nil {
		return err
	}
	engine, engineErr := ws.Updates()
	if engine == nil {
		return fmt.Errorf("the update methods cannot be built: %w", engineErr)
	}
	res, err := Resolve(cfg, engine, opts, now)
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
	case config.PhaseGA:
		return style.OK
	case config.PhaseEOM:
		return style.Warn
	case config.PhaseEOL:
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

// via is the method of a link and the key it updates.
func via(l Link) string {
	if key := l.Params["updateKey"]; key != "" {
		return l.Method + " (" + key + ")"
	}
	return l.Method
}

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
