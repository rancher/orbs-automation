package repolist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// fakeWorkspace answers from rows, and records what it was asked.
type fakeWorkspace struct {
	graph    *chain.Graph
	chainErr error
	rows     map[string]workspace.RepoPath // by slug; one not here is not found
	asked    []string
}

func (f *fakeWorkspace) Chain() (*chain.Graph, error) { return f.graph, f.chainErr }

func (f *fakeWorkspace) RepoPaths(slugs []string) []workspace.RepoPath {
	f.asked = slugs
	out := make([]workspace.RepoPath, len(slugs))
	for i, slug := range slugs {
		if r, ok := f.rows[slug]; ok {
			out[i] = r
		} else {
			out[i] = workspace.RepoPath{Slug: slug, Err: workspace.ErrRepoNotFound}
		}
	}
	return out
}

func found(slug, path string) workspace.RepoPath {
	return workspace.RepoPath{Slug: slug, Path: path, Source: workspace.PathScan}
}

// graph is five Projects: four of the organization and one external.
func newGraph(t *testing.T) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Doc("Project", "shell", "", "  repo: {name: rancher/shell, owned: true}\n"),
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher, owned: true}\n"),
		chaintest.Doc("Project", "charts", "", "  repo: {name: rancher/charts}\n"),
		chaintest.Doc("Project", "op", "", "  repo: {name: rancher/op, owned: true}\n"),
		chaintest.Doc("Project", "sdk", "", "  repo: {name: third-party/sdk, external: true}\n"),
	)
}

func everyoneExists(string) bool { return true }

func text(t *testing.T, ws *fakeWorkspace, exists func(string) bool) string {
	t.Helper()
	out, err := list(ws, exists, clicfg.OutputText, nil)
	require.NoError(t, err)
	return out
}

func TestTheClonedProjectsAreListedWithTheirPathSortedBySlug(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/shell":   found("rancher/shell", "/src/shell"),
		"rancher/rancher": found("rancher/rancher", "/src/rancher"),
	}}

	out := text(t, ws, everyoneExists)

	assert.Contains(t, out, "Cloned (2)\n  rancher/rancher  /src/rancher\n  rancher/shell    /src/shell\n")
}

func TestTheProjectsWithNoCloneAreListedByTheirSlug(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/rancher": found("rancher/rancher", "/src/rancher"),
	}}

	out := text(t, ws, everyoneExists)

	assert.Contains(t, out, "Not cloned (3)\n  rancher/charts\n  rancher/op\n  rancher/shell\n")
	assert.NotContains(t, out, "third-party/sdk", "an external project that is not cloned is not a gap")
}

func TestAProjectOutsideTheOrganizationIsListedWhenItIsCloned(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"third-party/sdk": found("third-party/sdk", "/src/sdk"),
	}}

	out := text(t, ws, everyoneExists)

	assert.Contains(t, out, "third-party/sdk  /src/sdk")
	assert.Contains(t, out, "0 of 4 projects cloned", "the count is of the projects that are expected, and an external one is not")
}

func TestAPathWhoseDirectoryIsMissingIsNotCloned(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/rancher": {Slug: "rancher/rancher", Path: "/src/gone", Source: workspace.PathSettings},
		"rancher/shell":   found("rancher/shell", "/src/shell"),
	}}
	exists := func(p string) bool { return p != "/src/gone" }

	out := text(t, ws, exists)

	assert.Contains(t, out, "Cloned (1)\n  rancher/shell  /src/shell\n")
	assert.Contains(t, out, "  rancher/rancher\n", "listed with the ones that have no clone")
	assert.NotContains(t, out, "/src/gone")
}

func TestAProjectWithSeveralClonesIsListedAsAmbiguousWithThePaths(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/op": {Slug: "rancher/op", Candidates: []string{"/src/op-a", "/src/op-b"}, Err: workspace.ErrRepoAmbiguous},
	}}

	out := text(t, ws, everyoneExists)

	assert.Contains(t, out, "Ambiguous (1)\n  rancher/op  /src/op-a, /src/op-b\n")
	assert.NotContains(t, out, "Not cloned (4)", "it has clones, so it is not a gap")
	assert.Contains(t, out, "Not cloned (3)")
}

func TestAnEmptySectionIsLeftOutAndTheSummaryCountsWhatIsCloned(t *testing.T) {
	all := map[string]workspace.RepoPath{}
	for _, s := range []string{"rancher/shell", "rancher/rancher", "rancher/charts", "rancher/op"} {
		all[s] = found(s, "/src/"+filepath.Base(s))
	}

	out := text(t, &fakeWorkspace{graph: newGraph(t), rows: all}, everyoneExists)

	assert.NotContains(t, out, "Not cloned")
	assert.NotContains(t, out, "Ambiguous")
	assert.Contains(t, out, "4 of 4 projects cloned\n")
}

func TestNothingClonedSaysSoAndListsEveryProject(t *testing.T) {
	out := text(t, &fakeWorkspace{graph: newGraph(t)}, everyoneExists)

	assert.NotContains(t, out, "Cloned (")
	assert.Contains(t, out, "Not cloned (4)")
	assert.Contains(t, out, "0 of 4 projects cloned\n")
}

func TestAConfigWithNoProjectsSaysSo(t *testing.T) {
	out := text(t, &fakeWorkspace{graph: chaintest.Projects(t)}, everyoneExists)

	assert.Equal(t, "No projects in the central config.\n", out)
}

func TestEveryProjectIsAskedAboutExternalOnesToo(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t)}

	text(t, ws, everyoneExists)

	assert.ElementsMatch(t, []string{"rancher/shell", "rancher/rancher", "rancher/charts", "rancher/op", "third-party/sdk"}, ws.asked)
}

func TestJSONHasThreeListsThatAreEmptyArraysAndNeverNull(t *testing.T) {
	ws := &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/rancher": found("rancher/rancher", "/src/rancher"),
	}}

	out, err := list(ws, everyoneExists, clicfg.OutputJSON, nil)

	require.NoError(t, err)
	var got Report
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, []Clone{{Slug: "rancher/rancher", Path: "/src/rancher"}}, got.Cloned)
	assert.Equal(t, []string{"rancher/charts", "rancher/op", "rancher/shell"}, got.NotCloned)
	assert.Contains(t, out, `"ambiguous": []`)
	assert.NotContains(t, out, "null")
}

func TestAFormatThatIsNotTextOrJSONIsRefused(t *testing.T) {
	_, err := list(&fakeWorkspace{graph: newGraph(t)}, everyoneExists, clicfg.OutputEnv, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "text or json")
}

func TestAConfigThatCannotBeReadIsAnError(t *testing.T) {
	_, err := list(&fakeWorkspace{chainErr: workspace.ErrNotFound}, everyoneExists, clicfg.OutputText, nil)

	require.True(t, errors.Is(err, workspace.ErrNotFound))
}

func TestTheRealCheckSeesADirectoryAndNothingElse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	assert.True(t, isDir(dir))
	assert.False(t, isDir(file))
	assert.False(t, isDir(filepath.Join(dir, "missing")))
}
