package workspace

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sccProducerDocs = "apiVersion: gero/v1alpha2\nkind: Project\nname: scc-operator\nspec:\n  repo: {name: rancher/scc-operator, owned: true}\n---\n" +
		"apiVersion: gero/v1alpha2\nkind: Stream\nname: scc-operator\nrefs: {project: scc-operator}\nspec: {universal: true}\n---\n" +
		"apiVersion: gero/v1alpha2\nkind: Artifact\nname: scc-image\nrefs: {stream: scc-operator}\nspec: {kind: image, name: rancher/scc-operator}\n"
	rancherHolding = "apiVersion: gero/v1alpha2\nkind: Holding\nname: rancher-holds-scc\nrefs: {project: rancher, artifact: scc-image}\n" +
		"spec:\n  at: {file: build.yaml, key: sccImage}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"
)

func bumpWorkspace(t *testing.T, extra map[string]string) *Workspace {
	t.Helper()
	files := map[string]string{
		"scc.yaml":     sccProducerDocs,
		"rancher.yaml": projectDocV2("rancher", rancherSlug) + "---\n" + rancherHolding,
	}
	for name, body := range extra {
		files[name] = body
	}
	dir := chainConfig(t, files)
	return Open(settings(t, "", nil, map[string]string{"config-dir": dir}), t.TempDir())
}

func TestBumpBuildsTheEngine(t *testing.T) {
	w := bumpWorkspace(t, nil)

	engine, err := w.Bump()

	require.NoError(t, err)
	producer, err := w.Project("scc-operator")
	require.NoError(t, err)
	routes := engine.RoutesFrom(producer)
	require.Len(t, routes, 1)
	assert.Equal(t, rancherSlug, routes[0].Consumer.Slug())
	require.Len(t, routes[0].Holdings, 1)
	assert.Equal(t, "rancher-holds-scc", routes[0].Holdings[0].Name())
}

func TestBumpAndChainShareOneGraph(t *testing.T) {
	w := bumpWorkspace(t, nil)

	g1, err := w.Chain()
	require.NoError(t, err)
	g2, err := w.Chain()
	require.NoError(t, err)

	assert.Same(t, g1, g2, "the config is read once for the invocation")
}

func TestBumpReportsWhatDependsOnAStrategy(t *testing.T) {
	w := bumpWorkspace(t, map[string]string{
		"edit.yaml": chaintest.Doc("Edit", "bad", "{holding: rancher-holds-scc}", "  strategy: nope\n"),
	})

	engine, err := w.Bump()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Contains(t, err.Error(), "chartsPackageBump", "it says what exists")
	assert.NotNil(t, engine, "the engine comes back with what was fine")
}

func TestBumpWithoutAConfigDirectory(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).Bump()

	assert.ErrorIs(t, err, ErrNotFound)
}
