package ship

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ConsumerHoldings is what one consumer of a project holds of its releases, branch
// by branch: the read half of a bump, with nothing written and nothing proposed.
// It is what gero repo status shows of who has picked a release up.
type ConsumerHoldings struct {
	Consumer string `json:"consumer"` // the repository, owner/name
	Name     string `json:"name"`     // its config name
	Method   string `json:"method"`   // the update method the project ships with
	// Error is why this consumer could not be traced at all: its branches could not
	// be read, or no branch or release could be worked out. The other consumers are
	// traced all the same.
	Error   string    `json:"error,omitempty"`
	Targets []Holding `json:"targets,omitempty"`
}

// Holding is one consumer branch that releases of the project go to.
type Holding struct {
	// Line is the Rancher line of the consumer's branch, "" when it is in none.
	Line   string `json:"line,omitempty"`
	Branch string `json:"branch"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	// Current is what the branch holds now, as the strategy reads it.
	Current string `json:"current,omitempty"`
	// Stable is how that stands against the latest stable release that goes to the
	// branch, and Newest against the newest release of any kind, which is the same
	// release when no prerelease is newer.
	Stable *Verdict `json:"stable,omitempty"`
	Newest *Verdict `json:"newest,omitempty"`
	// Error is why the branch could not be read: it is not on the host, or the
	// strategy found no value in its file.
	Error string `json:"error,omitempty"`
}

// Verdict is how what a branch holds stands against one release.
type Verdict struct {
	Tag      string                    `json:"tag"`  // the release
	Want     string                    `json:"want"` // what the branch would hold for it
	UpToDate bool                      `json:"upToDate"`
	Relation updatedispatcher.Relation `json:"relation"` // the release against what is held: newer means the branch is behind
}

// Holdings traces every consumer of producer: for each route out of it, the
// consumer's branches that its releases go to and what each holds, against the
// latest stable release and the newest of any kind. It uses the pieces a run of
// ship does, so the branches and the release of each are the ones ship would pick,
// and a consumer's file is read once for each branch. A consumer that cannot be
// traced has its Error set and does not stop the others. A producer that ships
// nowhere has none.
func Holdings(ctx context.Context, ws Reader, producer config.Project) ([]ConsumerHoldings, error) {
	engine, engineErr := ws.Updates()
	if engine == nil {
		return nil, engineErr
	}
	routes := engine.RoutesFrom(producer)
	if len(routes) == 0 {
		return nil, engineErr // a method of an edge that is not usable says why there is none
	}
	out := make([]ConsumerHoldings, len(routes))
	for i, route := range routes {
		out[i] = holdRoute(ctx, ws, engine, producer, route)
	}
	return out, nil
}

// holdRoute traces one route: what the consumer holds on each branch the
// producer's releases go to.
func holdRoute(ctx context.Context, ws Reader, engine *updatedispatcher.Engine, producer config.Project, route updatedispatcher.Route) ConsumerHoldings {
	out := ConsumerHoldings{Consumer: route.Consumer.Repo.Name, Name: route.Consumer.Name, Method: route.Method.Name}
	res, err := resolveRoute(ctx, ws, engine, producer, route)
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
			newestBy[sp.branch] = sp.release
		}
	}
	for _, sp := range stable.specs {
		out.Targets = append(out.Targets, res.holding(ctx, ws, sp, newestBy[sp.branch]))
	}
	return out
}

// holding reads one consumer branch once and says how what it holds stands against
// the release that goes to it and the newest one (zero when there is none).
func (r resolution) holding(ctx context.Context, ws Reader, sp spec, newest Release) Holding {
	h := Holding{Branch: sp.branch, Line: r.route.Consumer.LineOfBranch(sp.branch)}
	target, tree, err := r.head(ws, sp.branch)
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.Head = target.Commit
	tree = &memoTree{Tree: tree}
	chk, err := r.invocation.Check(ctx, tree, r.relOf(sp.release))
	if err != nil {
		h.Error = fmt.Sprintf("read %s of %s: %v", sp.branch, r.route.Consumer.Repo.Name, err)
		return h
	}
	h.Current = chk.Current
	h.Stable = verdict(sp.release, chk)
	switch {
	case newest.Tag == "":
	case newest.Tag == sp.release.Tag:
		same := *h.Stable
		h.Newest = &same
	default:
		if chkNewest, err := r.invocation.Check(ctx, tree, r.relOf(newest)); err == nil {
			h.Newest = verdict(newest, chkNewest)
		}
	}
	return h
}

func verdict(rel Release, chk updatedispatcher.Check) *Verdict {
	return &Verdict{Tag: rel.Tag, Want: chk.Want, UpToDate: chk.UpToDate, Relation: chk.Relation}
}

// memoTree reads each file of a tree once, so a branch can be compared with
// several releases for the price of one request.
type memoTree struct {
	updatedispatcher.Tree
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
	Method   string `json:"method"`   // the consumer's method it ships with
	// Error is why the source could not be traced, as for ConsumerHoldings.
	Error   string    `json:"error,omitempty"`
	Targets []Holding `json:"targets,omitempty"`
}

// Sources traces what the consumer takes: for each project that ships to it, the
// consumer's own branches that its releases go to, and what each holds against the
// source's latest stable release and its newest. Only this consumer is read, and
// not the others the source also ships to. A source that cannot be traced has its
// Error set and does not stop the others. A project nothing ships to has none.
func Sources(ctx context.Context, ws Reader, consumer config.Project) ([]SourceHoldings, error) {
	engine, engineErr := ws.Updates()
	if engine == nil {
		return nil, engineErr
	}
	routes := engine.RoutesInto(consumer)
	if len(routes) == 0 {
		return nil, engineErr
	}
	out := make([]SourceHoldings, len(routes))
	for i, in := range routes {
		held := holdRoute(ctx, ws, engine, in.Producer, in.Route)
		out[i] = SourceHoldings{Producer: in.Producer.Repo.Name, Name: in.Producer.Name, Method: held.Method, Error: held.Error, Targets: held.Targets}
	}
	return out, nil
}
