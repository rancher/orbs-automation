package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepsLink is the holding of scc-operator's image in rancher, followed by an edit
// with steps before the write and, after it, steps for some lines only.
func stepsLink() link {
	l := riLink()
	l.Consumer.Lines = []lineSpec{
		{Line: "v2.16"}, // development: main
		{Line: "v2.15", Branch: "release/v2.15"},
	}
	l.Pre = []stepSpec{{Run: "pre-method"}, {Run: "pre-edit", From: "producer"}}
	l.Post = []stepSpec{
		{Run: "post-method-new", Since: "v2.16"},
		{Run: "post-method-old", Until: "v2.15"},
		{Run: "post-edit"},
	}
	return l
}

func TestCheckPreviewListsTheStepsOfTheTargetBranchsLine(t *testing.T) {
	ws := loopFixture(t)
	ws.links = []link{stepsLink()}

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.NoError(t, err)
	byBranch := map[string]TargetPlan{}
	for _, tp := range got.Targets {
		byBranch[tp.Branch] = tp
	}
	old := byBranch["release/v2.15"]
	require.NotNil(t, old.Preview)
	assert.Contains(t, old.Preview.Body, "- Ran `post-method-old`")
	assert.NotContains(t, old.Preview.Body, "post-method-new")
	assert.Contains(t, old.Preview.Body, "- Ran `pre-edit`")
}

// routeTo is a route from scc-operator to a consumer, as the engine gives it, with
// no holdings: only who it is between matters to the choice.
func routeTo(t *testing.T, consumers ...string) []bump.Route {
	t.Helper()
	var pairs []string
	pairs = append(pairs, "scc-operator", "rancher/scc-operator")
	pairs = append(pairs, consumers...)
	g := chaintest.Projects(t, pairs...)
	var out []bump.Route
	for i := 0; i < len(consumers); i += 2 {
		out = append(out, bump.Route{Producer: g.Project("scc-operator"), Consumer: g.Project(consumers[i])})
	}
	return out
}

func TestPickRoute(t *testing.T) {
	routes := routeTo(t, "rancher", "rancher/rancher", "charts", "rancher/charts")
	producer := routes[0].Producer

	t.Run("the only route", func(t *testing.T) {
		got, err := pickRoute(producer, routes[:1], "")
		require.NoError(t, err)
		assert.Equal(t, "rancher", got.Consumer.Name())
	})
	t.Run("several consumers need --to", func(t *testing.T) {
		_, err := pickRoute(producer, routes, "")
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "--to")
	})
	t.Run("--to picks by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Charts"} {
			got, err := pickRoute(producer, routes, to)
			require.NoError(t, err, to)
			assert.Equal(t, "charts", got.Consumer.Name())
		}
	})
	t.Run("no route", func(t *testing.T) {
		_, err := pickRoute(producer, nil, "")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "holds a release")
		_, err = pickRoute(producer, routes[:1], "nobody")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "it ships to rancher", "it says where the producer does ship")
	})
}

func TestCheckPreviewHasTheStepCommitsOfTheTargetBranchsLineInTheOrderTheyRun(t *testing.T) {
	ws := loopFixture(t)
	ws.links = []link{stepsLink()}

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.NoError(t, err)
	var old TargetPlan
	for _, tp := range got.Targets {
		if tp.Branch == "release/v2.15" {
			old = tp
		}
	}
	require.NotNil(t, old.Preview)
	var runs []string
	var before []bool
	for _, s := range old.Preview.Steps {
		runs = append(runs, s.Run)
		before = append(before, s.BeforeEdit)
	}
	assert.Equal(t, []string{"pre-method", "pre-edit", "post-method-old", "post-edit"}, runs)
	assert.Equal(t, []bool{true, true, false, false}, before)
}
