package configvalidate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	dir    workspace.ConfigDir
	dirErr error
	cfg    config.Config
	cfgErr error
	// updatesErr is what the update dispatcher's engine finds wrong.
	updatesErr error
}

func (f fakeWorkspace) ConfigDir() (workspace.ConfigDir, error) { return f.dir, f.dirErr }
func (f fakeWorkspace) Config() (config.Config, error)          { return f.cfg, f.cfgErr }
func (f fakeWorkspace) Updates() (*updatedispatcher.Engine, error) {
	return nil, f.updatesErr
}

func TestRunSummarizesAValidConfig(t *testing.T) {
	ws := fakeWorkspace{
		dir: workspace.ConfigDir{Dir: "/work/config", Source: workspace.SourceCwd},
		cfg: config.Config{
			Projects:  make([]config.Project, 2),
			Externals: make([]config.External, 1),
			Lines:     &config.RancherLines{Lines: make([]config.Line, 5)},
		},
	}

	out, err := run(ws, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "/work/config")
	assert.Contains(t, out, "projects: 2")
	assert.Contains(t, out, "externals: 1")
	assert.Contains(t, out, "lines: 5")
	assert.True(t, strings.HasSuffix(out, "\n"), "output ends with a newline: %q", out)
}

func TestRunSummarizesAConfigWithoutLines(t *testing.T) {
	ws := fakeWorkspace{dir: workspace.ConfigDir{Dir: "/work/config"}, cfg: config.Config{}}

	out, err := run(ws, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "lines: 0")
}

func TestRunReturnsAMissingConfigDirectory(t *testing.T) {
	_, err := run(fakeWorkspace{dirErr: workspace.ErrNotFound}, nil)

	assert.ErrorIs(t, err, workspace.ErrNotFound)
}

func TestRunReturnsConfigProblems(t *testing.T) {
	problem := errors.New(`Project "a": repo.name "x" must be owner/name`)
	ws := fakeWorkspace{dir: workspace.ConfigDir{Dir: "/work/config"}, cfgErr: problem}

	out, err := run(ws, nil)

	assert.ErrorIs(t, err, problem)
	assert.Empty(t, out)
}

func TestRunOpensWorkspaceInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.project.yaml"), []byte(`apiVersion: gero/v1alpha1
kind: Project
name: a
spec:
  repo:
    name: o/a
`), 0o600))

	out, err := Run(clicfg.Settings{ConfigDir: dir}, "/elsewhere")

	require.NoError(t, err)
	assert.Contains(t, out, dir)
	assert.Contains(t, out, "projects: 1")
}

func TestRunWithoutCentralCheckout(t *testing.T) {
	_, err := Run(clicfg.Settings{}, t.TempDir())

	assert.ErrorIs(t, err, workspace.ErrNotFound)
}

func TestRunReturnsWhatTheUpdateDispatcherFinds(t *testing.T) {
	problem := errors.New(`Project "rancher": method "build-yaml-bump": strategy "nope" is not known: use one of yamlBump`)
	ws := fakeWorkspace{dir: workspace.ConfigDir{Dir: "/work/config"}, cfg: config.Config{}, updatesErr: problem}

	out, err := run(ws, nil)

	assert.ErrorIs(t, err, problem, "a method or an edge that cannot work is a config problem, like any other")
	assert.Empty(t, out)
}
