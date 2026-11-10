package graphresolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
)

type fakeWorkspace struct {
	g   *chain.Graph
	err error
}

func (f fakeWorkspace) Chain() (*chain.Graph, error) { return f.g, f.err }

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

// shipping is a config where shell produces an image that rancher, the final
// destination, holds, and scc-operator is a shared project outside any flow.
func shipping(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.13", "v2.12"),
		project("rancher", "  repo: {name: rancher/rancher, managedBy: rancher}\n  finalDestination: true\n"),
		project("shell", "  repo: {name: rancher/shell, managedBy: rancher}\n"),
		project("scc", "  repo: {name: rancher/scc-operator, managedBy: shared}\n"),
		project("upstream", "  repo: {name: kubernetes/upstream, external: true}\n"),
		chaintest.Doc("Stream", "shell", "{project: shell}", "  universal: true\n"),
		chaintest.Doc("Artifact", "shell-image", "{stream: shell}", "  kind: image\n  name: rancher/shell\n"),
		chaintest.Doc("Holding", "rancher-holds-shell", "{project: rancher, artifact: shell-image}",
			"  at: {file: build.yaml, key: shell}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all}\n"),
	)
}

func TestGraphDrawsEachProjectAsANode(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	for _, slug := range []string{"rancher/rancher", "rancher/shell", "rancher/scc-operator", "kubernetes/upstream"} {
		assert.Contains(t, out, `"`+slug+`": {`+"\n", slug)
	}
}

func TestGraphFlowsFromProducerThroughHoldingToHolder(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	assert.Contains(t, out, `"rancher/shell"."stream:shell"."artifact:shell-image" -> "holding:rancher-holds-shell"`)
	assert.Contains(t, out, `"holding:rancher-holds-shell" -> "rancher/rancher"`)
	assert.NotContains(t, out, `-> "rancher/shell"`, "the flow runs one way")
}

func TestGraphMarksStewardshipAndTheFinalDestination(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	assert.Contains(t, out, "\"rancher/shell\": {\n  class: [managed]")
	assert.Contains(t, out, "\"rancher/rancher\": {\n  class: [managed; hub]")
	assert.Contains(t, out, "\"rancher/scc-operator\": {\n  class: [shared]")
	assert.Contains(t, out, "\"kubernetes/upstream\": {\n  class: [external]")
}

func TestGraphLabelsTheHolderEdgeWithTheLinesTheHoldingServes(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	assert.Contains(t, out, `"holding:rancher-holds-shell" -> "rancher/rancher": "v2.13, v2.12"`)
}

func TestGraphIsTheSameEachTime(t *testing.T) {
	g := shipping(t)
	first, err := graph(fakeWorkspace{g: g})
	require.NoError(t, err)
	second, err := graph(fakeWorkspace{g: g})
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestGraphReturnsTheWorkspaceError(t *testing.T) {
	boom := errors.New("no config")
	_, err := graph(fakeWorkspace{err: boom})
	assert.ErrorIs(t, err, boom)
}

func TestGraphAsksForTheELKLayout(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	assert.Contains(t, out, "layout-engine: elk")
}

func TestGraphNestsTheStreamsAndArtifactsInTheirProject(t *testing.T) {
	out, err := graph(fakeWorkspace{g: shipping(t)})
	require.NoError(t, err)
	shell := out[strings.Index(out, `"rancher/shell": {`):]
	shell = shell[:strings.Index(shell, "\n}\n")]
	assert.Contains(t, shell, `"stream:shell": {`)
	assert.Contains(t, shell, `"artifact:shell-image": {class: [artifact]; label: "shell-image\nrancher/shell"}`)
	assert.NotContains(t, out[:strings.Index(out, `"rancher/shell": {`)], "stream:shell")
}

// twoSeries is a producer with a stream for each of two series whose artifacts
// publish under the same name, as a Go module does, held by one project twice.
func twoSeries(t *testing.T) string {
	t.Helper()
	g := chaintest.Load(t,
		project("lib", "  repo: {name: rancher/lib, managedBy: rancher}\n"),
		project("app", "  repo: {name: rancher/app, managedBy: rancher}\n"),
		chaintest.Doc("Stream", "lib-v1-1", "{project: lib}", "  tagPattern: \"v1.1.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "lib-v1-1-module", "{stream: lib-v1-1}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Stream", "lib-v1-2", "{project: lib}", "  tagPattern: \"v1.2.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "lib-v1-2-module", "{stream: lib-v1-2}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Holding", "app-holds-lib-1-1", "{project: app, artifact: lib-v1-1-module}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
		chaintest.Doc("Holding", "app-holds-lib-1-2", "{project: app, artifact: lib-v1-2-module}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n"),
	)
	out, err := graph(fakeWorkspace{g: g})
	require.NoError(t, err)
	return out
}

func TestGraphLabelsEachNodeApartFromOthersOfTheSameArtifactName(t *testing.T) {
	out := twoSeries(t)
	labels := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "label: "); i >= 0 && (strings.Contains(line, "artifact") || strings.Contains(line, "holding") || strings.Contains(line, "stream")) {
			label := line[i:]
			assert.False(t, labels[label], "two nodes are labelled %s", label)
			labels[label] = true
		}
	}
	assert.Contains(t, out, `label: "stream lib-v1-1\nv1.1.*"`)
	assert.Contains(t, out, `label: "lib-v1-2-module\ngithub.com/rancher/lib"`)
	assert.Contains(t, out, `label: "app-holds-lib-1-1\nrelease"`)
}

// withEdits is a project that holds two releases of one producer, with an edit
// that follows both and a strategy edit that follows one.
func withEdits(t *testing.T) string {
	t.Helper()
	hold := func(name, artifact string) string {
		return chaintest.Doc("Holding", name, "{project: app, artifact: "+artifact+"}",
			"  at: {file: go.mod, key: github.com/rancher/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all}\n")
	}
	g := chaintest.Load(t,
		project("lib", "  repo: {name: rancher/lib, managedBy: rancher}\n"),
		project("app", "  repo: {name: rancher/app, managedBy: rancher}\n"),
		chaintest.Doc("Stream", "lib", "{project: lib}", "  universal: true\n"),
		chaintest.Doc("Artifact", "lib-module", "{stream: lib}", "  kind: gomod\n  name: github.com/rancher/lib\n"),
		chaintest.Doc("Artifact", "lib-image", "{stream: lib}", "  kind: image\n  name: rancher/lib\n"),
		hold("app-holds-lib-module", "lib-module"),
		hold("app-holds-lib-image", "lib-image"),
		chaintest.Doc("Edit", "app-steps", "{holding: [app-holds-lib-module, app-holds-lib-image]}",
			"  preEdit:\n    - {run: scripts/prepare}\n  postEdit:\n    - {run: scripts/generate}\n    - {run: scripts/tidy}\n"),
		chaintest.Doc("Edit", "app-bump", "{holding: [app-holds-lib-module]}",
			"  strategy: chartsPackageBump\n  with: {package: lib}\n"),
	)
	out, err := graph(fakeWorkspace{g: g})
	require.NoError(t, err)
	return out
}

func TestGraphDrawsAnEditOnceWithAnEdgeFromEachHoldingItFollows(t *testing.T) {
	out := withEdits(t)
	assert.Equal(t, 1, strings.Count(out, `"edit:app-steps": {class:`))
	assert.Contains(t, out, `"holding:app-holds-lib-module" -> "edit:app-steps"`)
	assert.Contains(t, out, `"holding:app-holds-lib-image" -> "edit:app-steps"`)
	assert.Contains(t, out, `"holding:app-holds-lib-module" -> "edit:app-bump"`)
	assert.NotContains(t, out, `"holding:app-holds-lib-image" -> "edit:app-bump"`)
}

func TestGraphLabelsAnEditWithItsStrategyAndStepCounts(t *testing.T) {
	out := withEdits(t)
	assert.Contains(t, out, `"edit:app-steps": {class: [edit]; label: "app-steps\npre 1 step, post 2 steps"}`)
	assert.Contains(t, out, `"edit:app-bump": {class: [edit]; label: "app-bump\nstrategy chartsPackageBump"}`)
}
