package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stepsLink() link {
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

func TestCheckPreviewListsTheStepsOfTheTargetBranchsLine(t *testing.T) {
	ws := loopFixture()
	l := stepsLink()
	ws.links = []link{l}

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

func route(consumer, consumerSlug, method string) updatedispatcher.Route {
	return updatedispatcher.Route{
		Consumer: config.Project{Name: consumer, Repo: config.Repo{Name: consumerSlug}},
		Method:   &updatedispatcher.Method{ActionMethod: updatedispatcher.ActionMethod{Name: method}},
	}
}

func TestPickRoute(t *testing.T) {
	producer := config.Project{Name: "scc-operator"}
	build := route("rancher", "rancher/rancher", "build-yaml-bump")
	chart := route("rancher", "rancher/rancher", "chart-bump")
	other := route("charts", "rancher/charts", "build-yaml-bump")

	t.Run("the only route", func(t *testing.T) {
		got, err := pickRoute(producer, []updatedispatcher.Route{build}, "", "")
		require.NoError(t, err)
		assert.Equal(t, "build-yaml-bump", got.Method.Name)
	})
	t.Run("several consumers need --to", func(t *testing.T) {
		_, err := pickRoute(producer, []updatedispatcher.Route{build, other}, "", "")
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "--to")
	})
	t.Run("--to picks by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Charts"} {
			got, err := pickRoute(producer, []updatedispatcher.Route{build, other}, to, "")
			require.NoError(t, err, to)
			assert.Equal(t, "charts", got.Consumer.Name)
		}
	})
	t.Run("two methods of one consumer need --method", func(t *testing.T) {
		_, err := pickRoute(producer, []updatedispatcher.Route{build, chart}, "rancher", "")
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "--method")
		got, err := pickRoute(producer, []updatedispatcher.Route{build, chart}, "rancher", "chart-bump")
		require.NoError(t, err)
		assert.Equal(t, "chart-bump", got.Method.Name)
	})
	t.Run("no route", func(t *testing.T) {
		_, err := pickRoute(producer, nil, "", "")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "downstream")
		_, err = pickRoute(producer, []updatedispatcher.Route{build}, "nobody", "")
		assert.ErrorIs(t, err, ErrNoConsumer)
		assert.ErrorContains(t, err, "rancher (build-yaml-bump)", "it says where the producer does ship")
	})
}
