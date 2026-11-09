package configvalidate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	dir      workspace.CentralConfigDir
	dirErr   error
	graph    *chain.Graph
	chainErr error
}

func (f fakeWorkspace) CentralConfigDir() (workspace.CentralConfigDir, error) { return f.dir, f.dirErr }
func (f fakeWorkspace) Chain() (*chain.Graph, error)                          { return f.graph, f.chainErr }

func TestRunSummarizesAValidConfig(t *testing.T) {
	ws := fakeWorkspace{
		dir: workspace.CentralConfigDir{Dir: "/work/config", Source: workspace.SourceCwd},
		graph: chaintest.Load(t,
			chaintest.Lines("v2.16", "v2.15"),
			chaintest.Doc("Project", "a", "", "  repo: {name: acme/a}\n"),
			chaintest.Doc("Project", "b", "", "  repo: {name: acme/b}\n"),
			chaintest.Doc("Stream", "s", "{project: a}", "  universal: true\n"),
			chaintest.Doc("Artifact", "img", "{stream: s}", "  kind: image\n  name: acme/a\n"),
			chaintest.Doc("Holding", "h", "{project: b, artifact: img}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release}\n"),
		),
	}

	out, err := run(ws, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "/work/config")
	assert.Contains(t, out, "projects: 2")
	assert.Contains(t, out, "streams: 1")
	assert.Contains(t, out, "artifacts: 1")
	assert.Contains(t, out, "holdings: 1")
	assert.Contains(t, out, "lines: 2")
	assert.True(t, strings.HasSuffix(out, "\n"), "output ends with a newline: %q", out)
}

func TestRunSummarizesAConfigWithoutLines(t *testing.T) {
	ws := fakeWorkspace{dir: workspace.CentralConfigDir{Dir: "/work/config"}, graph: chaintest.Projects(t, "a", "acme/a")}

	out, err := run(ws, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "lines: 0")
}

func TestRunShowsEachWarningAndStillSucceeds(t *testing.T) {
	g, rep := chaintest.LoadFS(files(map[string]string{
		"lines.yaml": "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n",
		"p.yaml":     chaintest.Doc("Project", "p", "", "  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n"),
	}))
	require.NoError(t, rep.Err())

	out, err := run(fakeWorkspace{dir: workspace.CentralConfigDir{Dir: "/work/config"}, graph: g}, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "config OK")
	assert.Contains(t, out, "warning: p.yaml: Project \"p\": spec.branches.lines[0].branch: omitted")
	assert.Contains(t, out, "[default-branch-past-development]", "the warning names its rule, which is what a document acknowledges")
}

func TestRunReturnsAMissingConfigDirectory(t *testing.T) {
	_, err := run(fakeWorkspace{dirErr: workspace.ErrNotFound}, nil)

	assert.ErrorIs(t, err, workspace.ErrNotFound)
}

func TestRunReturnsConfigProblems(t *testing.T) {
	problem := errors.New(`Project "a": spec.repo.name "x" must be owner/name`)
	ws := fakeWorkspace{dir: workspace.CentralConfigDir{Dir: "/work/config"}, chainErr: problem}

	out, err := run(ws, nil)

	assert.ErrorIs(t, err, problem)
	assert.Empty(t, out)
}

func TestRunOpensWorkspaceInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(chaintest.Doc("Project", "a", "", "  repo: {name: o/a}\n")), 0o600))

	out, err := Run(clicfg.Settings{CentralConfigDir: dir}, "/elsewhere")

	require.NoError(t, err)
	assert.Contains(t, out, dir)
	assert.Contains(t, out, "projects: 1")
}

func TestRunWithoutCentralCheckout(t *testing.T) {
	_, err := Run(clicfg.Settings{}, t.TempDir())

	assert.ErrorIs(t, err, workspace.ErrNotFound)
}

// files is a file system of the given files, for chaintest.LoadFS.
func files(m map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range m {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}
