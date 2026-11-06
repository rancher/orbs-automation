package cdpath

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	projects []config.Project
	rows     map[string]workspace.RepoPath
	keys     []string
}

func (f *fakeWorkspace) Project(key string) (config.Project, error) {
	f.keys = append(f.keys, key)
	for _, p := range f.projects {
		if strings.EqualFold(p.Name, key) || strings.EqualFold(p.Repo.Name, key) {
			return p, nil
		}
	}
	return config.Project{}, fmt.Errorf("%q is %w", key, workspace.ErrUnknownProject)
}

func (f *fakeWorkspace) ConfiguredRepoPath(slug string) (workspace.RepoPath, error) {
	rp := f.rows[slug]
	return rp, rp.Err
}

func fixture() *fakeWorkspace {
	return &fakeWorkspace{
		projects: []config.Project{
			{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}},
			{Name: "rancher", Repo: config.Repo{Name: "rancher/rancher"}},
			{Name: "charts", Repo: config.Repo{Name: "rancher/charts"}},
			{Name: "shell", Repo: config.Repo{Name: "rancher/shell"}},
		},
		rows: map[string]workspace.RepoPath{
			"rancher/scc-operator": {Path: "/src/scc-operator", Source: workspace.PathSettings},
			"rancher/rancher":      {Path: "/src/rancher", Source: workspace.PathScan},
			"rancher/charts":       {Err: workspace.ErrRepoNotFound},
			"rancher/shell": {
				Candidates: []string{"/src/a/shell", "/src/b/shell"}, Err: workspace.ErrRepoAmbiguous},
		},
	}
}

func yes(string) bool { return true }

func TestPathByNameAndBySlug(t *testing.T) {
	for _, key := range []string{"scc-operator", "rancher/scc-operator", "SCC-Operator"} {
		out, err := path(fixture(), []string{key}, yes)

		require.NoError(t, err, key)
		assert.Equal(t, "/src/scc-operator\n", out, key)
	}
}

func TestPathFindsAClonedProjectUnderASearchDirectory(t *testing.T) {
	out, err := path(fixture(), []string{"rancher"}, yes)

	require.NoError(t, err)
	assert.Equal(t, "/src/rancher\n", out)
}

func TestPathNeedsAProjectAndNeverTheImplicitOne(t *testing.T) {
	for _, args := range [][]string{nil, {""}, {"."}, {"rancher", "charts"}} {
		ws := fixture()

		out, err := path(ws, args, yes)

		require.ErrorIs(t, err, ErrProjectRequired, "%q", args)
		assert.Empty(t, out)
		assert.Empty(t, ws.keys, "no lookup for %q", args)
	}
}

func TestPathUnknownProject(t *testing.T) {
	_, err := path(fixture(), []string{"nope"}, yes)

	require.ErrorIs(t, err, workspace.ErrUnknownProject)
}

func TestPathNoCloneSaysHowToFixIt(t *testing.T) {
	out, err := path(fixture(), []string{"charts"}, yes)

	require.ErrorIs(t, err, workspace.ErrRepoNotFound)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "rancher/charts")
	assert.Contains(t, err.Error(), "repos.paths.rancher/charts")
	assert.Contains(t, err.Error(), "repos.search")
}

func TestPathAmbiguousListsTheCandidates(t *testing.T) {
	_, err := path(fixture(), []string{"shell"}, yes)

	require.ErrorIs(t, err, workspace.ErrRepoAmbiguous)
	assert.Contains(t, err.Error(), "/src/a/shell, /src/b/shell")
	assert.Contains(t, err.Error(), "repos.paths.rancher/shell")
}

func TestPathThatIsNotADirectoryIsAnError(t *testing.T) {
	out, err := path(fixture(), []string{"scc-operator"}, func(string) bool { return false })

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "/src/scc-operator")
	assert.Contains(t, err.Error(), workspace.PathSettings)
}
