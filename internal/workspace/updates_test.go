package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const railsConsumer = "  updateMethods:\n" +
	"    - name: build-yaml-bump\n      strategy: yamlBump\n      with:\n        file: build.yaml\n" +
	"      allowedParams:\n        - key: updateKey\n          description: the key\n"

const sccProducer = "  downstream:\n    - to: rancher\n      method: build-yaml-bump\n      params:\n        updateKey: image\n"

func updatesWorkspace(t *testing.T, extra map[string]string) *Workspace {
	t.Helper()
	files := map[string]string{
		"scc.project.yaml":     projectYAML("scc-operator", "rancher/scc-operator") + sccProducer,
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) + railsConsumer,
	}
	for name, body := range extra {
		files[name] = body
	}
	dir := writeConfig(t, files)
	return Open(settings(t, "", nil, map[string]string{"config-dir": dir}), t.TempDir())
}

func TestUpdatesBuildsTheEngine(t *testing.T) {
	w := updatesWorkspace(t, nil)

	engine, err := w.Updates()

	require.NoError(t, err)
	m, ok := engine.Method("rancher", "build-yaml-bump")
	require.True(t, ok)
	assert.Equal(t, "yamlBump", m.StrategyName)

	producer, err := w.Project("scc-operator")
	require.NoError(t, err)
	routes := engine.RoutesFrom(producer)
	require.Len(t, routes, 1)
	assert.Equal(t, rancherSlug, routes[0].Consumer.Repo.Name)
	assert.Equal(t, "stable", routes[0].Method.Accept, "defaults are applied")
}

func TestUpdatesReportsWhatDependsOnTheStrategy(t *testing.T) {
	w := updatesWorkspace(t, map[string]string{
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) +
			"  updateMethods:\n    - name: build-yaml-bump\n      strategy: nope\n",
	})

	engine, err := w.Updates()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Contains(t, err.Error(), "yamlBump", "it says what exists")
	assert.NotNil(t, engine, "the engine comes back with what was fine, so one broken method does not stop another route")
}

func TestUpdatesWithoutAConfig(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).Updates()
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdatesAnEdgeThatNamesAParamTheMethodDoesNotAllow(t *testing.T) {
	w := updatesWorkspace(t, map[string]string{
		"scc.project.yaml": projectYAML("scc-operator", "rancher/scc-operator") +
			"  downstream:\n    - to: rancher\n      method: build-yaml-bump\n      params:\n        updateKey: image\n        nope: x\n",
	})
	_, err := w.Updates()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
}
