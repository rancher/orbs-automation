package ship

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/tagscheme"
)

// ConsumerHoldings is what one consumer of a project holds of its releases, branch
// by branch: the read half of a bump, with nothing written and nothing proposed.
// It is what gero repo status shows of who has picked a release up.
type ConsumerHoldings struct {
	Consumer string `json:"consumer"` // the repository, owner/name
	Name     string `json:"name"`     // its config name
	// Error is why this consumer could not be traced at all: its branches could not
	// be read, or no branch or release could be worked out. The other consumers are
	// traced all the same.
	Error   string    `json:"error,omitempty"`
	Targets []Holding `json:"targets,omitempty"`
}

// Holding is one consumer branch that releases of the project go to, and what each
// holding written there holds.
//
// TODO(release-state): two things gero repo status cannot say yet, both in
// docs/specs/REPO_STATUS.md under Not built yet.
//
//  1. How many releases the branch is behind. Verdict says only newer, same or
//     older. Count the producer's tags in the line's series that are newer than the
//     version held (the version is in Current, which the holding reads as the
//     value, so it needs the engine to hand back the version it found, as
//     Check.Relation already compares them), using the tags resolveRoute has read
//     (resolution.tags) and semver.InSeries. See the TODO on git.LatestReleases for
//     the per-series index to build once a second caller needs it.
//  2. The open PR that bumps the branch, so "behind" can say a bump is proposed.
//     gero ship finds it with pulls.Service (OpenForBase and OpenForHead) and the
//     marker (ParseMarker, decidePublish in publish.go); reuse those, as a tier of
//     its own behind a flag, since it costs a request for each branch and needs
//     the token a private repository needs.
type Holding struct {
	// Line is the Rancher line of the consumer's branch, "" when it is in none.
	Line   string `json:"line,omitempty"`
	Branch string `json:"branch"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	// Error is why the branch could not be read: it is not on the host.
	Error string `json:"error,omitempty"`
	// Held is what each holding of the branch holds, in the order the config reads
	// them.
	Held []HeldValue `json:"held,omitempty"`
}

// HeldValue is what one holding holds in a branch.
type HeldValue struct {
	Name string `json:"name"` // the holding
	File string `json:"file,omitempty"`
	// Current is what the branch holds now, as the holding reads it.
	Current string `json:"current,omitempty"`
	// Stage is the stage of the release the branch holds, for a producer whose tags
	// carry one (dev or stable), and empty otherwise.
	Stage string `json:"stage,omitempty"`
	// Stable is how that stands against the latest stable release that goes to the
	// branch, and Newest against the newest release of any kind, which is the same
	// release when no prerelease is newer.
	Stable *Verdict `json:"stable,omitempty"`
	Newest *Verdict `json:"newest,omitempty"`
	// Error is why the holding could not be read: its file is not in the branch, or
	// it holds no value at its place.
	Error string `json:"error,omitempty"`
	// Locations is each place the holding is read, for a holding with more than one.
	Locations []HeldLocation `json:"locations,omitempty"`
	// Differ says the locations hold different values, which the rule
	// locations-differ names. DifferReason is why the holding's document says that
	// is expected, when it acknowledges it.
	Differ       bool   `json:"differ,omitempty"`
	DifferReason string `json:"differReason,omitempty"`
}

// HeldLocation is what one location of a holding holds in a branch.
type HeldLocation struct {
	File    string `json:"file"`
	Current string `json:"current"`
}

// Verdict is how what a branch holds stands against one release.
type Verdict struct {
	Tag      string        `json:"tag"`  // the release
	Want     string        `json:"want"` // what the branch would hold for it
	UpToDate bool          `json:"upToDate"`
	Relation bump.Relation `json:"relation"` // the release against what is held: newer means the branch is behind
}

// Holdings traces every consumer of producer: for each route out of it, the
// consumer's branches that its releases go to and what each holding holds, against
// the latest stable release and the newest of any kind. It uses the pieces a run
// of ship does, so the branches and the release of each are the ones ship would
// pick, and a consumer's file is read once for each branch. A consumer that cannot
// be traced has its Error set and does not stop the others. A producer that ships
// nowhere has none.
func Holdings(ctx context.Context, ws Reader, producer *chain.Project) ([]ConsumerHoldings, error) {
	engine, engineErr := ws.Bump()
	if engine == nil {
		return nil, engineErr
	}
	routes := engine.RoutesFrom(producer)
	if len(routes) == 0 {
		return nil, engineErr // a holding or an edit that is not usable says why there is none
	}
	out := make([]ConsumerHoldings, len(routes))
	for i, route := range routes {
		out[i] = holdRoute(ctx, ws, route)
	}
	return out, nil
}

// holdRoute traces one route: what the consumer holds on each branch the
// producer's releases go to.
func holdRoute(ctx context.Context, ws Reader, route bump.Route) ConsumerHoldings {
	out := ConsumerHoldings{Consumer: route.Consumer.Slug(), Name: route.Consumer.Name()}
	res, err := resolveRoute(ctx, ws, route)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	stable, err := res.specs(Input{})
	if err != nil {
		out.Error = err.Error()
		return out
	}
	newestBy := map[string]Release{}
	if newest, err := res.specs(Input{Prerelease: true}); err == nil {
		for _, sp := range newest.specs {
			newestBy[sp.branch()] = sp.release
		}
	}
	for _, sp := range stable.specs {
		out.Targets = append(out.Targets, res.holding(ctx, ws, sp, newestBy[sp.branch()]))
	}
	return out
}

// holding reads one consumer branch once and says how what it holds stands against
// the release that goes to it and the newest one (zero when there is none).
func (r resolution) holding(ctx context.Context, ws Reader, sp spec, newest Release) Holding {
	h := Holding{Branch: sp.branch(), Line: sp.target.Line}
	target, tree, err := r.head(ws, sp.branch())
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.Head = target.Commit
	tree = &memoTree{Tree: tree}
	for _, held := range sp.target.Holdings {
		h.Held = append(h.Held, r.heldValue(ctx, tree, sp, held, newest))
	}
	return h
}

// heldValue reads one holding of a branch and says how it stands against the
// release that goes to the branch and the newest.
//
// TODO(locations): a holding with several locations shows only the first one's
// value and file (File, Current), and Verdict reads the relation of the first
// location that is behind. bump.Check already says when its locations hold
// different values (Check.Differ) and has each one in Check.Locations; show that
// on the row, so a branch whose nested modules are out of step is visible. See
// docs/specs/REPO_STATUS.md under Not built yet.
func (r resolution) heldValue(ctx context.Context, tree bump.Tree, sp spec, held *chain.Holding, newest Release) HeldValue {
	v := HeldValue{Name: held.Name()}
	one := bump.Target{Branch: sp.branch(), Line: sp.target.Line, Holdings: []*chain.Holding{held}}
	chk, err := r.route.Check(ctx, tree, one, sp.release.Tag, sp.release.Commit)
	if err != nil {
		v.Error = fmt.Sprintf("read %s of %s: %v", sp.branch(), r.route.Consumer.Slug(), err)
		return v
	}
	c := chk[0]
	v.File, v.Current = c.Change.Path, c.Current
	v.Stage = r.scheme.Stage(heldTag(c.Current))
	v.Stable = verdict(r.scheme, sp.release, c.Check)
	switch {
	case newest.Tag == "":
	case newest.Tag == sp.release.Tag:
		same := *v.Stable
		v.Newest = &same
	default:
		if chkNewest, err := r.route.Check(ctx, tree, one, newest.Tag, newest.Commit); err == nil {
			v.Newest = verdict(r.scheme, newest, chkNewest[0].Check)
		}
	}
	return v
}

// verdict is the check's answer for a release. When the holding could not order
// the value it read against the release, as it cannot for a tag that is no
// version, the producer's tag scheme does: the tag inside the value against the
// release's own.
func verdict(scheme tagscheme.Scheme, rel Release, chk bump.Check) *Verdict {
	v := &Verdict{Tag: rel.Tag, Want: chk.Want, UpToDate: chk.UpToDate, Relation: chk.Relation}
	held := heldTag(chk.Current)
	if v.Relation == bump.RelationUnknown && scheme.Valid(held) && scheme.Valid(rel.Tag) {
		switch c := scheme.Compare(rel.Tag, held); {
		case c > 0:
			v.Relation = bump.RelationNewer
		case c < 0:
			v.Relation = bump.RelationOlder
		default:
			v.Relation = bump.RelationSame
		}
	}
	return v
}

// heldTag is the tag in a value a holding read: what follows the last colon of an
// image reference such as rancher/rancher-assets:v2.16-20261030T1111Z-dev, or the
// whole value when it has none.
func heldTag(value string) string {
	if i := strings.LastIndex(value, ":"); i >= 0 {
		return value[i+1:]
	}
	return value
}

// memoTree reads each file of a tree once, so a branch can be compared with
// several releases for the price of one request.
type memoTree struct {
	bump.Tree
	files map[string]memoFile
}

type memoFile struct {
	data []byte
	err  error
}

func (m *memoTree) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if f, ok := m.files[path]; ok {
		return f.data, f.err
	}
	data, err := m.Tree.ReadFile(ctx, path)
	if m.files == nil {
		m.files = map[string]memoFile{}
	}
	m.files[path] = memoFile{data, err}
	return data, err
}

// SourceHoldings is what a project takes from one project that ships to it: the
// same read as ConsumerHoldings, seen from the consumer's end.
type SourceHoldings struct {
	Producer string `json:"producer"` // the repository, owner/name
	Name     string `json:"name"`     // its config name
	// Error is why the source could not be traced, as for ConsumerHoldings.
	Error   string    `json:"error,omitempty"`
	Targets []Holding `json:"targets,omitempty"`
}

// Sources traces what the consumer takes: for each project that ships to it, the
// consumer's own branches that its releases go to, and what each holding holds
// against the source's latest stable release and its newest. Only this consumer is
// read, and not the others the source also ships to. A source that cannot be traced
// has its Error set and does not stop the others. A project nothing ships to has
// none.
func Sources(ctx context.Context, ws Reader, consumer *chain.Project) ([]SourceHoldings, error) {
	engine, engineErr := ws.Bump()
	if engine == nil {
		return nil, engineErr
	}
	routes := engine.RoutesInto(consumer)
	if len(routes) == 0 {
		return nil, engineErr
	}
	out := make([]SourceHoldings, len(routes))
	for i, route := range routes {
		held := holdRoute(ctx, ws, route)
		out[i] = SourceHoldings{Producer: route.Producer.Slug(), Name: route.Producer.Name(), Error: held.Error, Targets: held.Targets}
	}
	return out, nil
}
