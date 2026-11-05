package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/git/repo/repotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectYAML(name, slug string) string {
	return "apiVersion: gero/v1alpha1\nkind: Project\nname: " + name + "\nspec:\n  repo:\n    name: " + slug + "\n"
}

// centralConfig writes a config directory with the rancher and charts Projects
// and returns it.
func centralConfig(t *testing.T) string {
	t.Helper()
	return writeConfig(t, map[string]string{
		"rancher.project.yaml": projectYAML("rancher", rancherSlug),
		"charts.project.yaml":  projectYAML("charts", "rancher/charts"),
	})
}

func projectWorkspace(t *testing.T, cwd string) *Workspace {
	t.Helper()
	return Open(settings(t, "", nil, map[string]string{"config-dir": centralConfig(t)}), cwd)
}

func TestProjectByName(t *testing.T) {
	p, err := projectWorkspace(t, t.TempDir()).Project("Rancher/Charts")
	require.NoError(t, err)
	assert.Equal(t, "rancher/charts", p.Repo.Name)
	assert.Equal(t, "main", p.Repo.DefaultBranch, "defaults are applied")
}

func TestProjectByDocumentName(t *testing.T) {
	ws := projectWorkspace(t, t.TempDir())
	for _, name := range []string{"charts", "Charts"} {
		p, err := ws.Project(name)
		require.NoError(t, err, name)
		assert.Equal(t, "rancher/charts", p.Repo.Name, "the name is the document's name, not the repository's")
	}
	_, err := ws.Project("rancher")
	assert.NoError(t, err, "the Project named rancher, though the slug rancher/rancher is longer")
}

func TestProjectNameDoesNotMatchTheRepositoryName(t *testing.T) {
	_, err := projectWorkspace(t, t.TempDir()).Project("nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
}

func TestProjectUnknownName(t *testing.T) {
	_, err := projectWorkspace(t, t.TempDir()).Project("rancher/nope")
	assert.ErrorIs(t, err, ErrUnknownProject)
	assert.ErrorContains(t, err, "rancher/nope")
}

func TestProjectFromWorkingDirectory(t *testing.T) {
	t.Run("upstream remote", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": forkURL, "upstream": upstreamURL})
		p, err := projectWorkspace(t, dir).Project("")
		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Repo.Name)
	})
	t.Run("only a fork remote", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": forkURL})
		p, err := projectWorkspace(t, dir).Project("")
		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Repo.Name)
	})
	t.Run("from a subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": upstreamURL})
		sub := filepath.Join(dir, "pkg", "x")
		require.NoError(t, os.MkdirAll(sub, 0o750))
		p, err := projectWorkspace(t, sub).Project("")
		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Repo.Name)
	})
	t.Run("an exact remote wins over a fork of another Project", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{
			"origin": "git@github.com:danpock/charts.git", "upstream": upstreamURL})
		p, err := projectWorkspace(t, dir).Project("")
		require.NoError(t, err)
		assert.Equal(t, rancherSlug, p.Repo.Name)
	})
}

func TestProjectWorkingDirectoryIsNotAProject(t *testing.T) {
	t.Run("unrelated remote", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": "https://github.com/someone/else.git"})
		_, err := projectWorkspace(t, dir).Project("")
		assert.ErrorIs(t, err, ErrNotProject)
	})
	t.Run("no remotes", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, nil)
		_, err := projectWorkspace(t, dir).Project("")
		assert.ErrorIs(t, err, ErrNotProject)
	})
	t.Run("not a repository", func(t *testing.T) {
		_, err := projectWorkspace(t, t.TempDir()).Project("")
		assert.ErrorIs(t, err, ErrNotProject)
	})
}

func TestProjectConfigErrors(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).Project("rancher/rancher")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestConfigRejectsADirectoryWithNoDocuments(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "config"), 0o750))
	_, err := Open(settings(t, "", nil, map[string]string{"config-dir": dir}), "/elsewhere").Config()
	assert.ErrorIs(t, err, ErrEmptyConfig)
}
