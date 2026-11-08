package configresolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git/repo/repotest"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorkspace answers RepoPaths from rows and records what it was asked.
type fakeWorkspace struct {
	graph    *chain.Graph
	chainErr error
	rows     map[string]workspace.RepoPath
	asked    [][]string
	// current is the slug of the Project the working directory is in, "" when it
	// is in none.
	current string
}

// Project is the real lookup: a key with a slash is a slug, any other a
// name, and "" or "." is the working directory's Project.
func (f *fakeWorkspace) Project(key string) (*chain.Project, error) {
	if f.chainErr != nil {
		return nil, f.chainErr
	}
	if key == "" || key == "." {
		for _, p := range f.graph.Projects() {
			if p.Slug() == f.current && f.current != "" {
				return p, nil
			}
		}
		return nil, workspace.ErrNotProject
	}
	if p := f.graph.Project(key); p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("%q is %w", key, workspace.ErrUnknownProject)
}

func (f *fakeWorkspace) Chain() (*chain.Graph, error) { return f.graph, f.chainErr }

func (f *fakeWorkspace) RepoPaths(slugs []string) []workspace.RepoPath {
	f.asked = append(f.asked, slugs)
	out := make([]workspace.RepoPath, len(slugs))
	for i, s := range slugs {
		out[i] = f.rows[s]
		out[i].Slug = s
	}
	return out
}

// graphOf is a config of plain projects named by their slugs.
func graphOf(t *testing.T, slugs ...string) *chain.Graph {
	t.Helper()
	var pairs []string
	for _, slug := range slugs {
		_, name, _ := strings.Cut(slug, "/")
		pairs = append(pairs, name, slug)
	}
	return chaintest.Projects(t, pairs...)
}

// mixed has one row of each outcome.
func mixed(t *testing.T) *fakeWorkspace {
	t.Helper()
	return &fakeWorkspace{
		graph: graphOf(t, "rancher/rancher", "rancher/scc-operator", "rancher/charts"),
		rows: map[string]workspace.RepoPath{
			"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathScan},
			"rancher/scc-operator": {
				Candidates: []string{"/src/op-a", "/src/op-b"}, Err: workspace.ErrRepoAmbiguous},
			"rancher/charts": {Err: workspace.ErrRepoNotFound},
		},
	}
}

func TestReposTextTable(t *testing.T) {
	out, err := repos(mixed(t), clicfg.OutputText, nil, nil)

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
	ws := mixed(t)
	ws.graph = graphOf(t, "rancher/charts") // not found

	out, _ := repos(ws, clicfg.OutputText, nil, nil)

	assert.Contains(t, out, "not found: clone it")
	assert.NotContains(t, out, "ambiguous")
}

func TestReposAllResolvedIsNotAnError(t *testing.T) {
	ws := mixed(t)
	ws.graph = graphOf(t, "rancher/rancher")

	out, err := repos(ws, clicfg.OutputText, nil, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "/src/rancher")
	assert.NotContains(t, out, "repos.paths", "no hints when nothing went wrong")
}

func TestReposJSON(t *testing.T) {
	out, err := repos(mixed(t), clicfg.OutputJSON, nil, nil)

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
	out, err := repos(&fakeWorkspace{graph: graphOf(t)}, clicfg.OutputJSON, nil, nil)

	require.NoError(t, err)
	assert.JSONEq(t, "[]", out)
}

func TestReposAsksForProjectsOnly(t *testing.T) {
	ws := mixed(t)
	ws.graph = chaintest.Load(t,
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher}\n"),
		chaintest.Doc("Project", "scc-operator", "", "  repo: {name: rancher/scc-operator}\n"),
		chaintest.Doc("Project", "charts", "", "  repo: {name: rancher/charts}\n"),
		chaintest.Doc("Project", "kubernetes", "", "  repo: {name: kubernetes/kubernetes, external: true}\n"),
	)

	_, _ = repos(ws, clicfg.OutputText, nil, nil)

	assert.Equal(t, [][]string{{"rancher/rancher", "rancher/scc-operator", "rancher/charts"}}, ws.asked)
}

func TestReposOneSlug(t *testing.T) {
	ws := mixed(t)

	out, err := repos(ws, clicfg.OutputText, []string{"rancher/rancher"}, nil)

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
	assert.Contains(t, out, "/src/rancher")
	assert.NotContains(t, out, "scc-operator")
}

func TestReposOneSlugUsesTheConfigSpelling(t *testing.T) {
	ws := mixed(t)

	_, err := repos(ws, clicfg.OutputText, []string{"Rancher/Rancher"}, nil)

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
}

func TestReposUnknownSlug(t *testing.T) {
	ws := mixed(t)

	out, err := repos(ws, clicfg.OutputText, []string{"rancher/nope"}, nil)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnresolved)
	assert.Contains(t, err.Error(), "rancher/nope")
	assert.Empty(t, out)
	assert.Empty(t, ws.asked)
}

func TestReposByProjectName(t *testing.T) {
	ws := mixed(t)
	out, err := repos(ws, clicfg.OutputText, []string{"scc-operator"}, nil)
	assert.ErrorIs(t, err, ErrUnresolved, "that clone is ambiguous in the fixture")
	assert.Equal(t, [][]string{{"rancher/scc-operator"}}, ws.asked, "a name is asked for by its slug")
	assert.Contains(t, out, "ambiguous")
}

func TestReposDotIsTheProjectTheWorkingDirectoryIsIn(t *testing.T) {
	ws := mixed(t)
	ws.current = "rancher/rancher"
	out, err := repos(ws, clicfg.OutputText, []string{"."}, nil)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"rancher/rancher"}}, ws.asked)
	assert.Contains(t, out, "/src/rancher")
}

func TestReposDotOutsideAProject(t *testing.T) {
	ws := mixed(t)
	out, err := repos(ws, clicfg.OutputText, []string{"."}, nil)
	assert.ErrorIs(t, err, workspace.ErrNotProject)
	assert.Empty(t, out)
	assert.Empty(t, ws.asked)
}

func TestReposOneSlugUnresolved(t *testing.T) {
	out, err := repos(mixed(t), clicfg.OutputText, []string{"rancher/charts"}, nil)

	assert.ErrorIs(t, err, ErrUnresolved)
	assert.Contains(t, out, "not found")
}

func TestReposConfigErrors(t *testing.T) {
	for name, cfgErr := range map[string]error{
		"no config directory": workspace.ErrNotFound,
		"invalid config":      errors.New(`Project "a": repo.name "x" must be owner/name`),
	} {
		t.Run(name, func(t *testing.T) {
			ws := &fakeWorkspace{chainErr: cfgErr}

			out, err := repos(ws, clicfg.OutputText, nil, nil)

			assert.ErrorIs(t, err, cfgErr)
			assert.Empty(t, out)
			assert.Empty(t, ws.asked)
		})
	}
}

func TestReposRejectsOtherFormats(t *testing.T) {
	for _, format := range []string{clicfg.OutputEnv, "yaml"} {
		t.Run(format, func(t *testing.T) {
			ws := mixed(t)

			out, err := repos(ws, format, nil, nil)

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
		require.NoError(t, os.WriteFile(filepath.Join(central, name+".yaml"), []byte(chaintest.Doc("Project", name, "", "  repo: {name: "+slug+"}\n")), 0o600))
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
