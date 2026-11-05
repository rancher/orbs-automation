package workspace

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/git/repo/repotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const railsConsumer = "  updateMethods:\n" +
	"    - name: build-yaml-bump\n      strategy: yamlBump\n      with:\n        file: build.yaml\n" +
	"      allowedParams:\n        - key: updateKey\n          description: the key\n"

const sccProducer = "  downstream:\n    - to: rancher\n      method: build-yaml-bump\n      params:\n        updateKey: image\n"

func linksWorkspace(t *testing.T) *Workspace {
	t.Helper()
	dir := writeConfig(t, map[string]string{
		"scc.project.yaml":     projectYAML("scc-operator", "rancher/scc-operator") + sccProducer,
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) + railsConsumer,
		"charts.project.yaml":  projectYAML("charts", "rancher/charts"),
	})
	return Open(settings(t, "", nil, map[string]string{"config-dir": dir}), t.TempDir())
}

func TestLinks(t *testing.T) {
	producer, links, err := linksWorkspace(t).Links("scc-operator")

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", producer.Repo.Name)
	require.Len(t, links, 1)
	assert.Equal(t, rancherSlug, links[0].Consumer.Repo.Name)
	assert.Equal(t, "build-yaml-bump", links[0].Method.Name)
	assert.Equal(t, "yamlBump", links[0].Method.Strategy)
	assert.Equal(t, "image", links[0].Edge.Params["updateKey"])
	assert.Equal(t, "stable", links[0].Method.Accept.When, "defaults are applied")
}

func TestLinksBySlug(t *testing.T) {
	producer, links, err := linksWorkspace(t).Links("rancher/scc-operator")
	require.NoError(t, err)
	assert.Equal(t, "scc-operator", producer.Name)
	assert.Len(t, links, 1)
}

func TestLinksOfAProjectThatShipsNowhere(t *testing.T) {
	_, links, err := linksWorkspace(t).Links("charts")
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestLinksUnknownProject(t *testing.T) {
	_, _, err := linksWorkspace(t).Links("nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
}

func TestLinksFromTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	repotest.NewRepoAt(t, dir, map[string]string{"upstream": "https://github.com/rancher/scc-operator.git"})
	cfg := writeConfig(t, map[string]string{
		"scc.project.yaml":     projectYAML("scc-operator", "rancher/scc-operator") + sccProducer,
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) + railsConsumer,
	})
	w := Open(settings(t, "", nil, map[string]string{"config-dir": cfg}), dir)

	producer, links, err := w.Links("")

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", producer.Repo.Name)
	assert.Len(t, links, 1)
}

func TestLinksFromAWorkingDirectoryThatIsNoProject(t *testing.T) {
	_, _, err := linksWorkspace(t).Links("")
	assert.ErrorIs(t, err, ErrNotProject)
}
