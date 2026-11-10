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
	assert.Contains(t, shell, `"artifact:shell-image": {class: [artifact]; label: "rancher/shell"}`)
	assert.NotContains(t, out[:strings.Index(out, `"rancher/shell": {`)], "stream:shell")
}
