package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindParamsPrecedence(t *testing.T) {
	link := riLink("yamlBump")
	link.Edge.Params = map[string]string{"updateKey": "k"} // no value: the method's default applies

	got := bindParams(link)
	assert.Equal(t, "k", got["updateKey"])
	assert.Equal(t, "{tag}", got["value"], "the default the method offers, which is also the strategy's")
	assert.Equal(t, "build.yaml", got["file"], "what the method fixes")

	link.Method.AllowedParams = []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{version}"}}
	assert.Equal(t, "{version}", bindParams(link)["value"], "a default the method sets beats the strategy's")

	link.Edge.Params["value"] = "{image}:{tag}"
	assert.Equal(t, "{image}:{tag}", bindParams(link)["value"], "what the edge passes beats any default")
}

func TestBindParamsAFixedValueBeatsTheEdge(t *testing.T) {
	link := riLink("yamlBump")
	link.Edge.Params["file"] = "elsewhere.yaml" // validation rejects this; binding still never lets it win
	assert.Equal(t, "build.yaml", bindParams(link)["file"])
}

func TestBindParamsKeepsParamsOnlyTheStepsUse(t *testing.T) {
	link := riLink("yamlBump")
	link.Method.AllowedParams = append(link.Method.AllowedParams, config.AllowedParam{Key: "reviewers", Default: "orbs"})
	got := bindParams(link)
	assert.Equal(t, "orbs", got["reviewers"], "a param the strategy does not know is for the steps")
}

func stepsLink() config.Link {
	link := riLink("yamlBump")
	link.Consumer.Release.Lines = []config.ProjectLine{
		{Line: "v2.16"}, // development: main
		{Line: "v2.15", Branch: "release/v2.15"},
	}
	link.Method.PreEdit = []config.Step{{Run: config.Command{"pre-method"}}}
	link.Method.PostEdit = []config.Step{
		{Run: config.Command{"post-method-new"}, Since: "v2.16"},
		{Run: config.Command{"post-method-old"}, Until: "v2.15"},
	}
	link.Edge.PreEdit = []config.Step{{Run: config.Command{"pre-edge"}, From: "producer"}}
	link.Edge.PostEdit = []config.Step{{Run: config.Command{"post-edge"}}}
	return link
}

func programs(steps []config.Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Run[0]
	}
	return out
}

func TestStepsRunOutermostLast(t *testing.T) {
	r := resolution{link: stepsLink()}

	assert.Equal(t, []string{"pre-method", "pre-edge", "post-edge", "post-method-new"}, programs(r.steps("main")),
		"the consumer's pre-edit first and post-edit last, around the producer's, for the development line")
	assert.Equal(t, []string{"pre-method", "pre-edge", "post-edge", "post-method-old"}, programs(r.steps("release/v2.15")),
		"the other line takes the other post-edit step")
}

func TestStepsForABranchWithNoLineRunEverythingThatIsNotRanged(t *testing.T) {
	r := resolution{link: stepsLink()}
	got := programs(r.steps("feature/x"))
	assert.Equal(t, []string{"pre-method", "pre-edge", "post-edge", "post-method-new", "post-method-old"}, got,
		"nothing is skipped on a guess: a branch in no line gets every step")
}

func TestCheckPreviewListsTheStepsOfTheTargetBranchsLine(t *testing.T) {
	ws := loopFixture()
	link := stepsLink()
	ws.links = []config.Link{link}

	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})

	require.NoError(t, err)
	byBranch := map[string]TargetPlan{}
	for _, tp := range got.Targets {
		byBranch[tp.Branch] = tp
	}
	old := byBranch["release/v2.15"]
	require.NotNil(t, old.Preview)
	assert.Contains(t, old.Preview.Body, "- Ran `post-method-old`")
	assert.NotContains(t, old.Preview.Body, "post-method-new")
	assert.Contains(t, old.Preview.Body, "- Ran `pre-edge`")
}

func TestPickLink(t *testing.T) {
	producer := config.Project{Name: "scc-operator"}
	build := riLink("yamlBump")
	chart := riLink("yamlBump")
	chart.Method.Name = "chart-bump"
	chart.Edge.Method = "chart-bump"
	other := riLink("yamlBump")
	other.Consumer.Name, other.Consumer.Repo.Name = "charts", "rancher/charts"

	t.Run("the only route", func(t *testing.T) {
		got, err := pickLink(producer, []config.Link{build}, "", "")
		require.NoError(t, err)
		assert.Equal(t, "build-yaml-bump", got.Method.Name)
	})
	t.Run("several consumers need --to", func(t *testing.T) {
		_, err := pickLink(producer, []config.Link{build, other}, "", "")
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "--to")
	})
	t.Run("--to picks by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Charts"} {
			got, err := pickLink(producer, []config.Link{build, other}, to, "")
			require.NoError(t, err, to)
			assert.Equal(t, "charts", got.Consumer.Name)
		}
	})
	t.Run("two methods of one consumer need --method", func(t *testing.T) {
		_, err := pickLink(producer, []config.Link{build, chart}, "rancher", "")
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "--method")
		got, err := pickLink(producer, []config.Link{build, chart}, "rancher", "chart-bump")
		require.NoError(t, err)
		assert.Equal(t, "chart-bump", got.Method.Name)
	})
	t.Run("no route", func(t *testing.T) {
		_, err := pickLink(producer, nil, "", "")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "downstream")
		_, err = pickLink(producer, []config.Link{build}, "nobody", "")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "rancher (build-yaml-bump)", "it says where the producer does ship")
	})
}
