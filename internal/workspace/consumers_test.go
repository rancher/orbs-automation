package workspace

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/git/repo/repotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func consumersWorkspace(t *testing.T) *Workspace {
	t.Helper()
	dir := writeConfig(t, map[string]string{
		"scc.project.yaml": projectYAML("scc-operator", "rancher/scc-operator"),
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) +
			"  sources:\n    - from: scc-operator\n      update:\n        strategy: buildYamlBump\n",
		"charts.project.yaml": projectYAML("charts", "rancher/charts"),
	})
	return Open(settings(t, "", nil, map[string]string{"config-dir": dir}), t.TempDir())
}

func TestConsumers(t *testing.T) {
	producer, consumers, err := consumersWorkspace(t).Consumers("scc-operator")

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", producer.Repo.Name)
	require.Len(t, consumers, 1)
	assert.Equal(t, rancherSlug, consumers[0].Project.Repo.Name)
	assert.Equal(t, "scc-operator", consumers[0].Source.From)
	assert.Equal(t, "buildYamlBump", consumers[0].Source.Update.Strategy)
	assert.Equal(t, "stable", consumers[0].Source.Accept.When, "defaults are applied")
}

func TestConsumersBySlug(t *testing.T) {
	producer, consumers, err := consumersWorkspace(t).Consumers("rancher/scc-operator")
	require.NoError(t, err)
	assert.Equal(t, "scc-operator", producer.Name)
	assert.Len(t, consumers, 1)
}

func TestConsumersOfAProjectNobodyTakesFrom(t *testing.T) {
	_, consumers, err := consumersWorkspace(t).Consumers("charts")
	require.NoError(t, err)
	assert.Empty(t, consumers)
}

func TestConsumersUnknownProject(t *testing.T) {
	_, _, err := consumersWorkspace(t).Consumers("nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
}

func TestConsumersFromTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	repotest.NewRepoAt(t, dir, map[string]string{"origin": "git@github.com:danpock/scc-operator.git", "upstream": "https://github.com/rancher/scc-operator.git"})
	cfg := writeConfig(t, map[string]string{
		"scc.project.yaml": projectYAML("scc-operator", "rancher/scc-operator"),
		"rancher.project.yaml": projectYAML("rancher", rancherSlug) +
			"  sources:\n    - from: scc-operator\n      update:\n        strategy: buildYamlBump\n",
	})
	w := Open(settings(t, "", nil, map[string]string{"config-dir": cfg}), dir)

	producer, consumers, err := w.Consumers("")

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", producer.Repo.Name)
	assert.Len(t, consumers, 1)
}

func TestConsumersFromAWorkingDirectoryThatIsNoProject(t *testing.T) {
	_, _, err := consumersWorkspace(t).Consumers("")
	assert.ErrorIs(t, err, ErrNotProject)
}
