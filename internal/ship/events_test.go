package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder is a Sink that keeps the events it is given.
type recording struct{ events []Event }

func (r *recording) Event(e Event) { r.events = append(r.events, e) }

func (r *recording) kinds() []EventKind {
	out := make([]EventKind, len(r.events))
	for i, e := range r.events {
		out[i] = e.Kind
	}
	return out
}

// of returns the events of a kind.
func (r *recording) of(kind EventKind) []Event {
	var out []Event
	for _, e := range r.events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// first is the first event of a kind, which the test requires.
func (r *recording) first(t *testing.T, kind EventKind) Event {
	t.Helper()
	events := r.of(kind)
	require.NotEmpty(t, events, "there is a %q event in %v", kind, r.kinds())
	return events[0]
}

// twoTargets is a run with release/v2.15 to commit, with a step that makes a
// second commit, and release/v2.14 already up to date.
func twoTargets(t *testing.T) (*realWorkspace, *Executor) {
	t.Helper()
	ws, _, ex := newShip(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.14")
	put(t, ws.up, "build.yaml", newImage)
	runGit(t, ws.up, "commit", "--quiet", "-am", "already there")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	ws.links[0].Method.PostEdit = []config.Step{{Run: config.Command{"./scripts/gen.sh"}}}
	return ws, ex
}

func TestShipEmitsTheEventsInTheOrderTheWorkHappens(t *testing.T) {
	ws, ex := twoTargets(t)
	rec := &recording{}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn(""), ex, rec)

	require.NoError(t, err)
	assert.Equal(t, []EventKind{
		EventResolved, EventTargetChecked, EventTargetChecked, EventTargetsChecked, EventImagesChecked,
		EventTargetStarted, EventCommit, EventCommit, EventTargetDone, EventDone,
	}, rec.kinds())
	resolved := rec.first(t, EventResolved)
	require.NotNil(t, resolved.Plan)
	assert.Equal(t, "rancher/scc-operator", resolved.Plan.Producer)
	assert.Equal(t, "v0.5.3", resolved.Plan.Release.Tag)
	assert.Equal(t, "rancher/rancher", resolved.Plan.Consumer)
	assert.Equal(t, "build-yaml-bump", resolved.Plan.Method)
	assert.Empty(t, resolved.Plan.Targets, "the header only: no branch has been read")

	checked := rec.of(EventTargetChecked)
	require.Len(t, checked, 2)
	require.NotNil(t, checked[0].Target)
	require.NotNil(t, checked[1].Target)
	assert.Equal(t, "release/v2.15", checked[0].Target.Branch)
	assert.Equal(t, StatusReady, checked[0].Target.Status)
	assert.Equal(t, "build.yaml", checked[0].Target.File, "what the file and the wanted value are is known from the first read")
	assert.Equal(t, "rancher/scc-operator:v0.5.3", checked[0].Target.Want)
	assert.Equal(t, "release/v2.14", checked[1].Target.Branch)
	assert.Equal(t, StatusUpToDate, checked[1].Target.Status)

	require.NotNil(t, rec.first(t, EventImagesChecked).Images)
	started := rec.first(t, EventTargetStarted)
	assert.Equal(t, "release/v2.15", started.Branch)
	assert.Equal(t, InPlace, started.Place)
	commits := rec.of(EventCommit)
	require.Len(t, commits, 2)
	require.NotNil(t, commits[0].Commit)
	require.NotNil(t, commits[1].Commit)
	assert.Equal(t, "Update scc-operator to v0.5.3", commits[0].Commit.Subject)
	assert.Equal(t, "Run `./scripts/gen.sh`", commits[1].Commit.Subject)
	assert.Equal(t, "release/v2.15", commits[0].Branch)
	done := rec.first(t, EventTargetDone)
	require.NotNil(t, done.Result)
	assert.Equal(t, ResultCommitted, done.Result.Status)
	assert.Equal(t, "release/v2.15", done.Branch)

	final := rec.first(t, EventDone)
	require.NotNil(t, final.Plan)
	assert.Equal(t, plan.Status, final.Plan.Status)
	require.Len(t, final.Plan.Targets, 2)
	assert.NotNil(t, final.Plan.Targets[0].Result, "the whole plan, with the results")
}

func TestShipWithoutAnExecutorEmitsOnlyTheCheck(t *testing.T) {
	ws, _ := twoTargets(t)
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, shipIn(""), nil, rec)

	require.NoError(t, err)
	assert.Equal(t, []EventKind{
		EventResolved, EventTargetChecked, EventTargetChecked, EventTargetsChecked, EventImagesChecked, EventDone,
	}, rec.kinds())
}

func TestShipABlockedReleaseEmitsNoWork(t *testing.T) {
	ws, ex := twoTargets(t)
	p := &fakeProber{results: map[string]registry.Result{
		"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: registry.NotFound},
	}}
	rec := &recording{}

	_, err := Ship(bg, ws, p, shipIn(""), ex, rec)

	require.NoError(t, err)
	assert.Equal(t, []EventKind{
		EventResolved, EventTargetChecked, EventTargetChecked, EventTargetsChecked, EventImagesChecked, EventDone,
	}, rec.kinds())
	require.NotNil(t, rec.first(t, EventDone).Plan)
	assert.NotEmpty(t, rec.first(t, EventDone).Plan.Blockers)
}

func TestShipAFailureAfterACommitStillEndsTheTarget(t *testing.T) {
	ws, ex := twoTargets(t)
	ws.links[0].Method.PostEdit = []config.Step{{Run: config.Command{"./scripts/gen.sh"}}, {Run: config.Command{"false"}}}
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, shipIn("release/v2.15"), ex, rec)

	require.NoError(t, err)
	assert.Equal(t, []EventKind{
		EventResolved, EventTargetChecked, EventTargetsChecked, EventImagesChecked,
		EventTargetStarted, EventCommit, EventCommit, EventTargetDone, EventDone,
	}, rec.kinds(), "the commits made before the failure were reported as they were made")
	require.NotNil(t, rec.first(t, EventTargetDone).Result)
	assert.Equal(t, ResultFailed, rec.first(t, EventTargetDone).Result.Status)
}

func TestShipAnErrorBeforeThePlanEmitsNoDone(t *testing.T) {
	ws, ex := twoTargets(t)
	ws.projectErr = assert.AnError
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, shipIn(""), ex, rec)

	require.Error(t, err)
	assert.NotContains(t, rec.kinds(), EventDone)
}

func TestShipTargetsThatCouldNotBeReadAreStillReported(t *testing.T) {
	ws, ex := twoTargets(t)
	runGit(t, ws.up, "branch", "-D", "release/v2.14")
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, shipIn(""), ex, rec)

	require.NoError(t, err)
	checked := rec.of(EventTargetChecked)
	require.Len(t, checked, 2, "a failed target is an event too")
	require.NotNil(t, checked[1].Target)
	assert.Equal(t, StatusFailed, checked[1].Target.Status)
}

func TestShipEveryEventAboutATargetNamesItsLineAndAPushNamesTheBranch(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(101, "danpock", "v0.5.2")}}
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, rec)

	require.NoError(t, err)
	about := map[EventKind]bool{
		EventTargetChecked: true, EventDecided: true, EventTargetStarted: true, EventCommit: true,
		EventPushed: true, EventPROpened: true, EventSuperseded: true, EventTargetDone: true,
	}
	seen := map[EventKind]bool{}
	for _, e := range rec.events {
		if about[e.Kind] {
			seen[e.Kind] = true
			assert.Equal(t, "v2.15", e.Line, "%s names the line of release/v2.15", e.Kind)
			assert.Equal(t, "release/v2.15", e.Branch, "and still the branch")
		}
	}
	assert.Len(t, seen, len(about), "every kind was emitted: %v", rec.kinds())
	assert.Equal(t, botV215, rec.first(t, EventPushed).NewBranch, "which branch was pushed")
}

func TestShipARefreshEventsNameTheLineToo(t *testing.T) {
	ws, _, ex, _, _, in := refreshing(t)
	withGenerate(ws)
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, rec)

	require.NoError(t, err)
	refreshed := rec.first(t, EventRefreshed)
	assert.Equal(t, "v2.15", refreshed.Line)
	assert.Equal(t, botV215, rec.first(t, EventPushed).NewBranch)
}
