package configresolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git/repo/repotest"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorkspace answers RepoPaths from rows and records what it was asked.
type fakeWorkspace struct {
	cfg    config.Config
	cfgErr error
	rows   map[string]workspace.RepoPath
	asked  [][]string
	// current is the slug of the Project the working directory is in, "" when it
	// is in none.
	current string
}

// Project is the real lookup: a key with a slash is a slug, any other a name,
// and "" or "." is the working directory's Project.
func (f *fakeWorkspace) Project(key string) (config.Project, error) {
	if f.cfgErr != nil {
		return config.Project{}, f.cfgErr
	}
	if key == "" || key == "." {
		for _, p := range f.cfg.Projects {
			if p.Repo.Name == f.current && f.current != "" {
				return p, nil
			}
		}
		return config.Project{}, workspace.ErrNotProject
	}
	for _, p := range f.cfg.Projects {
		if (strings.Contains(key, "/") && strings.EqualFold(p.Repo.Name, key)) ||
			(!strings.Contains(key, "/") && strings.EqualFold(p.Name, key)) {
			return p, nil
		}
	}
	return config.Project{}, fmt.Errorf("%q is %w", key, workspace.ErrUnknownProject)
}

func (f *fakeWorkspace) Config() (config.Config, error) { return f.cfg, f.cfgErr }

func (f *fakeWorkspace) RepoPaths(slugs []string) []workspace.RepoPath {
	f.asked = append(f.asked, slugs)
	out := make([]workspace.RepoPath, len(slugs))
	for i, s := range slugs {
		out[i] = f.rows[s]
		out[i].Slug = s
	}
	return out
}

func project(slug string) config.Project {
	_, name, _ := strings.Cut(slug, "/")
	return config.Project{Name: name, Repo: config.Repo{Name: slug}}
}

// mixed has one row of each outcome.
func mixed() *fakeWorkspace {
	return &fakeWorkspace{
		cfg: config.Config{Projects: []config.Project{
			project("rancher/rancher"), project("rancher/scc-operator"), project("rancher/charts"),
		}},
		rows: map[string]workspace.RepoPath{
			"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathScan},
			"rancher/scc-operator": {
				Candidates: []string{"/src/op-a", "/src/op-b"}, Err: workspace.ErrRepoAmbiguous},
			"rancher/charts": {Err: workspace.ErrRepoNotFound},
		},
	}
}

func TestReposTextTable(t *testing.T) {
	out, err := repos(mixed(), clicfg.OutputText, nil)

	require.ErrorIs(t, err, ErrUnresolved)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	require.Len(t, lines, 7, out) // header, 3 rows, blank, 2 hints
	assert.Equal(t, []string{"SLUG", "PATH", "SOURCE"}, strings.Fields(lines[0]))
	assert.Equal(t, []string{"rancher/rancher", "/src/rancher", "repos.search", "scan"}, strings.Fields(lines[1]))
	assert.Equal(t, []string{"rancher/scc-operator", "(ambiguous:", "/src/op-a,", "/src/op-b)", "-"}, strings.Fields(lines[2]))
	assert.Equal(t, []string{"rancher/charts", "(not", "found)", "-"}, strings.Fields(lines[3]))
	assert.Empty(t, lines[4])
	assert.Equal(t, "not found: clone it under a repos.search directory, or set repos.paths.<slug>", lines[5])
	assert.Equal(t, "ambiguous: set repos.paths.<slug> to choose one", lines[6])
}

func TestReposTextHintsOnlyForWhatWentWrong(t *testing.T) {
	ws := mixed()
	ws.cfg.Projects = ws.cfg.Projects[2:] // rancher/charts, not found

	out, _ := repos(ws, clicfg.OutputText, nil)

	assert.Contains(t, out, "not found: clone it")
	assert.NotContains(t, out, "ambiguous")
}

func TestReposAllResolvedIsNotAnError(t *testing.T) {
	ws := mixed()
	ws.cfg.Projects = ws.cfg.Projects[:1]

	out, err := repos(ws, clicfg.OutputText, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "/src/rancher")
	assert.NotContains(t, out, "repos.paths", "no hints when nothing went wrong")
}

func TestReposJSON(t *testing.T) {
	out, err := repos(mixed(), clicfg.OutputJSON, nil)

	require.ErrorIs(t, err, ErrUnresolved)
	var got []struct {
		Slug       string   `json:"slug"`
		Path       string   `json:"path"`
		Source     string   `json:"source"`
		Candidates []string `json:"candidates"`
		Error      string   `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	require.Len(t, got, 3)
	assert.Equal(t, "rancher/rancher", got[0].Slug)
	assert.Equal(t, "/src/rancher", got[0].Path)
	assert.Equal(t, "repos.search scan", got[0].Source)
	assert.Empty(t, got[0].Error)
	assert.Equal(t, []string{"/src/op-a", "/src/op-b"}, got[1].Candidates)
	assert.Equal(t, workspace.ErrRepoAmbiguous.Error(), got[1].Error)
	assert.Empty(t, got[1].Path)
	assert.Equal(t, workspace.ErrRepoNotFound.Error(), got[2].Error)
}

func TestReposNoProjects(t *testing.T) {
	out, err := repos(&fakeWorkspace{}, clicfg.OutputJSON, nil)

	require.NoError(t, err)
	assert.JSONEq(t, "[]", out)
}

func TestReposAsksForProjectsOnly(t *testing.T) {
	ws := mixed()
	ws.cfg.Externals = []config.External{{Name: "kubernetes", Repo: config.Repo{Name: "kubernetes/kubernetes"}}}

	_, _ = repos(ws, clicfg.OutputText, nil)

	assert.Equal(t, [][]string{{"rancher/rancher", "rancher/scc-operator", "rancher/charts"}}, ws.asked)
}

func TestReposOneSlug(t *testing.T) {
	ws := mixed()

	out, err := repos(ws, clicfg.OutputText, []string{"rancher/rancher"})

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
	assert.Contains(t, out, "/src/rancher")
	assert.NotContains(t, out, "scc-operator")
}

func TestReposOneSlugUsesTheConfigSpelling(t *testing.T) {
	ws := mixed()

	_, err := repos(ws, clicfg.OutputText, []string{"Rancher/Rancher"})

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
}

func TestReposUnknownSlug(t *testing.T) {
	ws := mixed()

	out, err := repos(ws, clicfg.OutputText, []string{"rancher/nope"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnresolved)
	assert.Contains(t, err.Error(), "rancher/nope")
	assert.Empty(t, out)
	assert.Empty(t, ws.asked)
}

func TestReposByProjectName(t *testing.T) {
	ws := mixed()
	out, err := repos(ws, clicfg.OutputText, []string{"scc-operator"})
	assert.ErrorIs(t, err, ErrUnresolved, "that clone is ambiguous in the fixture")
	assert.Equal(t, [][]string{{"rancher/scc-operator"}}, ws.asked, "a name is asked for by its slug")
	assert.Contains(t, out, "ambiguous")
}

func TestReposDotIsTheProjectTheWorkingDirectoryIsIn(t *testing.T) {
	ws := mixed()
	ws.current = "rancher/rancher"
	out, err := repos(ws, clicfg.OutputText, []string{"."})
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
	assert.Contains(t, out, "/src/rancher")
}

func TestReposDotOutsideAProject(t *testing.T) {
	ws := mixed()
	out, err := repos(ws, clicfg.OutputText, []string{"."})
	assert.ErrorIs(t, err, workspace.ErrNotProject)
	assert.Empty(t, out)
	assert.Empty(t, ws.asked)
}

func TestReposOneSlugUnresolved(t *testing.T) {
	out, err := repos(mixed(), clicfg.OutputText, []string{"rancher/charts"})

	assert.ErrorIs(t, err, ErrUnresolved)
	assert.Contains(t, out, "not found")
}

func TestReposConfigErrors(t *testing.T) {
	for name, cfgErr := range map[string]error{
		"no config directory": workspace.ErrNotFound,
		"invalid config":      errors.New(`Project "a": repo.name "x" must be owner/name`),
	} {
		t.Run(name, func(t *testing.T) {
			ws := &fakeWorkspace{cfgErr: cfgErr}

			out, err := repos(ws, clicfg.OutputText, nil)

			assert.ErrorIs(t, err, cfgErr)
			assert.Empty(t, out)
			assert.Empty(t, ws.asked)
		})
	}
}

func TestReposRejectsOtherFormats(t *testing.T) {
	for _, format := range []string{clicfg.OutputEnv, "yaml"} {
		t.Run(format, func(t *testing.T) {
			ws := mixed()

			out, err := repos(ws, format, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), format)
			assert.Empty(t, out)
			assert.Empty(t, ws.asked)
		})
	}
}

func TestReposEndToEnd(t *testing.T) {
	central, search := t.TempDir(), t.TempDir()
	for _, slug := range []string{"rancher/rancher", "rancher/charts"} {
		_, name, _ := strings.Cut(slug, "/")
		require.NoError(t, os.WriteFile(filepath.Join(central, name+".project.yaml"), []byte(`apiVersion: gero/v1alpha1
kind: Project
name: `+name+`
spec:
  repo:
    name: `+slug+`
`), 0o600))
	}
	repotest.NewRepoAt(t, filepath.Join(search, "rancher-src"), map[string]string{"upstream": "https://github.com/rancher/rancher.git"})
	s := clicfg.Settings{
		ConfigDir: central,
		Output:    clicfg.OutputText,
		Repos:     clicfg.Repos{Search: []string{search}},
	}

	out, err := Repos(s, t.TempDir(), nil)

	require.ErrorIs(t, err, ErrUnresolved, out)
	assert.Contains(t, out, filepath.Join(search, "rancher-src"))
	assert.Contains(t, out, "rancher/charts")
	assert.Contains(t, out, "not found")
}

func TestReposWithoutCentralCheckout(t *testing.T) {
	_, err := Repos(clicfg.Settings{Output: clicfg.OutputText}, t.TempDir(), nil)

	assert.ErrorIs(t, err, workspace.ErrNotFound)
}
