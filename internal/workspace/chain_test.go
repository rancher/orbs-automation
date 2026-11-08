package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git/repo/repotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectDocV2(name, slug string) string {
	return "apiVersion: gero/v1alpha2\nkind: Project\nname: " + name + "\nspec:\n  repo: {name: " + slug + "}\n"
}

// chainConfig writes a config directory with the documents in its v1alpha2
// folder, and returns the directory.
func chainConfig(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, chainDir)
	require.NoError(t, os.Mkdir(sub, 0o750))
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(sub, name), []byte(body), 0o600))
	}
	return dir
}

func chainWorkspace(t *testing.T, cwd string) *Workspace {
	t.Helper()
	dir := chainConfig(t, map[string]string{
		"rancher.yaml": projectDocV2("rancher", rancherSlug),
		"charts.yaml":  projectDocV2("charts", "rancher/charts"),
	})
	return Open(settings(t, "", nil, map[string]string{"config-dir": dir}), cwd)
}

func TestChainLoadsAndJoinsTheConfig(t *testing.T) {
	dir := chainConfig(t, map[string]string{"a.yaml": projectDocV2("a", "o/a")})

	g, err := open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere", nil).Chain()

	require.NoError(t, err)
	require.Len(t, g.Projects(), 1)
	assert.Equal(t, "main", g.Project("a").DefaultBranch(), "defaults are applied")
}

func TestChainFindsTheCentralCheckoutInTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "config", chainDir), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "a.project.yaml"), []byte(minimalProject), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", chainDir, "a.yaml"), []byte(projectDocV2("a", "o/a")), 0o600))

	g, err := Open(settings(t, "", nil, nil), root).Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 1)
}

func TestChainWithoutAConfigDirectory(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).Chain()

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChainErrorNamesTheDirectoryAndTheFile(t *testing.T) {
	dir := chainConfig(t, map[string]string{"bad.yaml": projectDocV2("p", "nope")})

	_, err := open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere", nil).Chain()

	require.Error(t, err)
	assert.Contains(t, err.Error(), dir)
	assert.Contains(t, err.Error(), "bad.yaml")
	assert.Contains(t, err.Error(), "spec.repo.name")
}

func TestChainRejectsADirectoryWithNoDocuments(t *testing.T) {
	dir := chainConfig(t, nil)

	_, err := open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere", nil).Chain()

	assert.ErrorIs(t, err, ErrEmptyConfig)
}

func TestChainWithNoV1alpha2Folder(t *testing.T) {
	dir := t.TempDir()

	_, err := open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere", nil).Chain()

	require.Error(t, err)
	assert.Contains(t, err.Error(), chainDir)
}

func TestChainLoadsTheCheckedInConfig(t *testing.T) {
	dir, err := filepath.Abs("../../config")
	require.NoError(t, err)

	g, err := open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere", nil).Chain()

	require.NoError(t, err)
	assert.NotEmpty(t, g.Projects())
}

func TestChainProjectByNameOrSlug(t *testing.T) {
	ws := chainWorkspace(t, t.TempDir())

	for _, key := range []string{"charts", "Charts", "Rancher/Charts"} {
		p, err := ws.ChainProject(key)
		require.NoError(t, err, key)
		assert.Equal(t, "rancher/charts", p.Slug(), key)
	}
	_, err := ws.ChainProject("nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
	_, err = ws.ChainProject("rancher/nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
	assert.ErrorContains(t, err, "rancher/nope")
}

func TestChainProjectFromTheWorkingDirectory(t *testing.T) {
	t.Run("upstream remote", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": forkURL, "upstream": upstreamURL})

		p, err := chainWorkspace(t, dir).ChainProject("")

		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Slug())
	})
	t.Run("only a fork remote, and a dot", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": forkURL})

		p, err := chainWorkspace(t, dir).ChainProject(".")

		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Slug())
	})
	t.Run("an exact remote wins over a fork of another Project", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": "git@github.com:danpock/charts.git", "upstream": upstreamURL})

		p, err := chainWorkspace(t, dir).ChainProject("")

		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Slug())
	})
	t.Run("an unrelated remote", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": "https://github.com/someone/else.git"})

		_, err := chainWorkspace(t, dir).ChainProject("")

		assert.ErrorIs(t, err, ErrNotProject)
	})
	t.Run("not a repository", func(t *testing.T) {
		_, err := chainWorkspace(t, t.TempDir()).ChainProject("")

		assert.ErrorIs(t, err, ErrNotProject)
	})
}

func TestChainProjectConfigErrors(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).ChainProject("rancher/rancher")

	assert.ErrorIs(t, err, ErrNotFound)
}
