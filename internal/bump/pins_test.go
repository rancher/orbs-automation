package bump

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinGraph is wrangler (wr), a Go module that pins lasso (la), another, and bro,
// which holds wrangler in go.mod and, from v2.16, in cmd/tool/go.mod. A Holding of
// lasso is virtual: the pin makes it in the same two files.
func pinGraph(t *testing.T, extra ...string) *chain.Graph {
	t.Helper()
	docs := []string{
		chaintest.Lines("v2.16", "v2.15", "v2.14"),
		chaintest.Doc("Project", "wr", "", "  repo: {name: acme/wr, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "la", "", "  repo: {name: acme/la, managedBy: frameworks}\n"),
		chaintest.Doc("Project", "bro", "", "  repo: {name: acme/bro, managedBy: orbs}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}, {line: v2.14, branch: r14}]\n"),
		chaintest.Doc("Stream", "wr", "{project: wr}", "  universal: true\n  tagPattern: \"v1.*\"\n"),
		chaintest.Doc("Stream", "la", "{project: la}", "  universal: true\n  tagPattern: \"v0.2.*\"\n"),
		chaintest.Doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		chaintest.Doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		chaintest.Doc("Holding", "bro-holds-wr", "{project: bro, artifact: wr-mod}", "  at:\n    - {file: go.mod, key: github.com/acme/wr, format: gomod}\n    - {file: cmd/tool/go.mod, key: github.com/acme/wr, format: gomod, lines: {since: v2.16}}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {since: v2.15}}\n"),
	}
	return chaintest.Load(t, append(docs, extra...)...)
}

const (
	broRoot = "module acme/bro\n\nrequire (\n\tgithub.com/acme/la v0.2.3\n\tgithub.com/acme/wr v1.7.1\n)\n"
	broTool = "module acme/bro/cmd/tool\n\nrequire (\n\tgithub.com/acme/la v0.2.3 // indirect\n\tgithub.com/acme/wr v1.7.1\n)\n"
	// broNoLasso is a go.mod that requires wrangler and not lasso.
	broNoLasso = "module acme/bro/cmd/tool\n\nrequire github.com/acme/wr v1.7.1\n"
)

func virtualOf(t *testing.T, g *chain.Graph) (*chain.Holding, Release) {
	t.Helper()
	require.Len(t, g.Virtual(), 1)
	v := g.Virtual()[0]
	rel, err := ReleaseOf(v, "v0.2.5", "")
	require.NoError(t, err)
	return v, rel
}

func TestCheckOfAVirtualHoldingWantsTheVersionAPinResolves(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))

	got, err := CheckHolding(bg, files{"go.mod": broRoot, "cmd/tool/go.mod": broTool}, v, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.False(t, got.UpToDate)
	assert.Equal(t, "v0.2.5", got.Want)
	assert.Equal(t, "v0.2.3", got.Current)
	assert.Equal(t, RelationNewer, got.Relation)
	require.Len(t, got.Locations, 2, "both files require lasso")
	assert.Len(t, got.Changes(), 2)
	assert.False(t, got.HeldNewer)
}

func TestCheckOfAVirtualHoldingIsAFloor(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))
	newer := "module acme/bro\n\nrequire github.com/acme/la v0.2.9\n"

	got, err := CheckHolding(bg, files{"go.mod": newer, "cmd/tool/go.mod": newer}, v, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.UpToDate, "a newer version holds what the pin needs")
	assert.True(t, got.HeldNewer, "and the check says it is newer than the pin resolves")
	assert.Empty(t, got.Changes())

	same, err := CheckHolding(bg, files{"go.mod": "module m\n\nrequire github.com/acme/la v0.2.5\n", "cmd/tool/go.mod": "module m\n\nrequire github.com/acme/la v0.2.5\n"}, v, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.True(t, same.UpToDate)
	assert.False(t, same.HeldNewer, "the same is not newer")
}

func TestCheckOfAVirtualHoldingOneLocationNewerAndOneOlderWantsTheOlder(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))

	got, err := CheckHolding(bg, files{"go.mod": "module m\n\nrequire github.com/acme/la v0.2.9\n", "cmd/tool/go.mod": broTool}, v, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.False(t, got.UpToDate)
	require.Len(t, got.Changes(), 1)
	assert.Equal(t, "cmd/tool/go.mod", got.Changes()[0].Path)
	assert.True(t, got.HeldNewer, "one file holds more than the pin needs")
}

func TestCheckOfAVirtualHoldingSkipsAFileThatDoesNotRequireTheModule(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))

	got, err := CheckHolding(bg, files{"go.mod": broRoot, "cmd/tool/go.mod": broNoLasso}, v, "v2.16", "main", rel)

	require.NoError(t, err)
	require.Len(t, got.Locations, 1, "the file that does not require lasso has nothing to hold")
	assert.Equal(t, "go.mod", got.Locations[0].Place.File)
}

func TestCheckOfAVirtualHoldingWhoseFilesRequireNothingHoldsNothing(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))
	none := "module m\n\nrequire github.com/acme/wr v1.7.1\n"

	got, err := CheckHolding(bg, files{"go.mod": none, "cmd/tool/go.mod": broNoLasso}, v, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.Absent)
	assert.True(t, got.UpToDate, "there is nothing to write")
	assert.Empty(t, got.Locations)
}

func TestCheckOfARealHoldingStillFailsForAFileThatDoesNotRequireTheModule(t *testing.T) {
	g := pinGraph(t)
	h := g.Holdings()[0]
	rel, err := ReleaseOf(h, "v1.7.4", "")
	require.NoError(t, err)

	_, err = CheckHolding(bg, files{"go.mod": "module m\n"}, h, "v2.15", "r15", rel)

	require.Error(t, err, "only a virtual Holding skips it")
}

func TestApplyOfAVirtualHoldingRaisesWhatIsOlderAndKeepsWhatIsNewer(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))
	tree := files{"go.mod": "module m\n\nrequire github.com/acme/la v0.2.9\n", "cmd/tool/go.mod": broTool}

	got, err := ApplyHolding(bg, tree, v, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.Changed)
	require.Len(t, got.Changes, 1)
	assert.Equal(t, "cmd/tool/go.mod", got.Changes[0].Path)
	assert.Contains(t, tree["go.mod"], "v0.2.9", "a newer version is not lowered")
	assert.Contains(t, tree["cmd/tool/go.mod"], "github.com/acme/la v0.2.5 // indirect", "an older one is raised, and its comment kept")
}

func TestApplyOfAVirtualHoldingSkipsAFileThatDoesNotRequireTheModule(t *testing.T) {
	v, rel := virtualOf(t, pinGraph(t))
	tree := files{"go.mod": broRoot, "cmd/tool/go.mod": broNoLasso}

	got, err := ApplyHolding(bg, tree, v, "v2.16", "main", rel)

	require.NoError(t, err)
	require.Len(t, got.Changes, 1)
	assert.Equal(t, broNoLasso, tree["cmd/tool/go.mod"], "the file is as it was")
}

// pinRoute is the route wrangler to bro, and the target for a line, with the
// companions of its holding in it as a caller that derived their releases makes it.
func pinRoute(t *testing.T) (Route, Target) {
	t.Helper()
	g := pinGraph(t)
	e, err := NewEngine(Builtin(), g)
	require.NoError(t, err)
	routes := e.RoutesFrom(g.Project("wr"))
	require.Len(t, routes, 1)
	return routes[0], Target{Branch: "main", Line: "v2.16", Holdings: routes[0].Holdings}
}

func TestCompanionsAreTheVirtualHoldingsThatFollowTheTargetsHoldings(t *testing.T) {
	route, target := pinRoute(t)

	got := route.Companions(target)

	require.Len(t, got, 1)
	assert.Equal(t, "bro-holds-wr+la-mod", got[0].Name())
	assert.Empty(t, route.Companions(Target{Branch: "r14", Line: "v2.14", Holdings: route.Holdings}), "the holding is not for v2.14, so neither is what it pins")
	assert.Len(t, route.Companions(Target{Branch: "x", Holdings: route.Holdings}), 1, "a branch in no line gets them all")
}

func TestCheckOfATargetWithCompanionsChecksEachAgainstItsOwnRelease(t *testing.T) {
	route, target := pinRoute(t)
	companions := route.Companions(target)
	target.Holdings = append(slices.Clone(target.Holdings), companions...)
	target.Pinned = map[*chain.Holding]string{companions[0]: "v0.2.5"}
	tree := files{"go.mod": broRoot, "cmd/tool/go.mod": broTool}

	got, err := route.Check(bg, tree, target, "v1.7.4", "sha")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "v1.7.4", got[0].Want, "wrangler, the tag of the run")
	assert.Equal(t, "v0.2.5", got[1].Want, "lasso, the tag the pin derived")
	assert.Equal(t, "v0.2.3", got[1].Current)
}

func TestCheckOfAVirtualHoldingWithNoDerivedReleaseIsAnError(t *testing.T) {
	route, target := pinRoute(t)
	target.Holdings = append(slices.Clone(target.Holdings), route.Companions(target)...)

	_, err := route.Check(bg, files{"go.mod": broRoot, "cmd/tool/go.mod": broTool}, target, "v1.7.4", "sha")

	require.Error(t, err)
	assert.ErrorContains(t, err, "no release was derived")
}

func TestRunWritesTheCompanionsWithTheirReleasesAndTidiesOnce(t *testing.T) {
	g := pinGraph(t, chaintest.Doc("Edit", "tidy", "{holding: bro-holds-wr}", "  postEdit:\n    - {run: tidy-it}\n"))
	e, err := NewEngine(Builtin(), g)
	require.NoError(t, err)
	route := e.RoutesFrom(g.Project("wr"))[0]
	target := Target{Branch: "main", Line: "v2.16", Holdings: route.Holdings}
	companions := route.Companions(target)
	target.Holdings = append(slices.Clone(target.Holdings), companions...)
	target.Pinned = map[*chain.Holding]string{companions[0]: "v0.2.5"}
	tree := files{"go.mod": broRoot, "cmd/tool/go.mod": broTool}
	var order []string
	rec := &recorder{order: &order}

	res, err := route.Run(bg, tree, rec, target, "v1.7.4", "sha", "/work", func(_ context.Context, u Unit) error {
		order = append(order, fmt.Sprintf("unit %s %d", u.Kind, len(u.Changes)))
		return nil
	})

	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, []string{"unit edit 4", "run tidy-it", "unit step 0"}, order, "both holdings are one write, then the Edit's step runs once")
	assert.Contains(t, tree["go.mod"], "github.com/acme/wr v1.7.4")
	assert.Contains(t, tree["go.mod"], "github.com/acme/la v0.2.5")
	assert.Contains(t, tree["cmd/tool/go.mod"], "github.com/acme/la v0.2.5 // indirect")
}

func TestAVirtualHoldingIsUnusableWhenTheHoldingItComesFromIs(t *testing.T) {
	g := pinGraph(t, chaintest.Doc("Edit", "bad", "{holding: bro-holds-wr}", "  strategy: nope\n"))

	e, err := NewEngine(Builtin(), g)

	require.Error(t, err)
	assert.Empty(t, e.RoutesFrom(g.Project("wr")), "the pinning holding is not usable")
	assert.Empty(t, e.RoutesFrom(g.Project("la")), "and nor is what it makes")
}
