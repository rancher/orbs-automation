package configshow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	dir workspace.CentralConfigDir
	err error
}

func (f fakeWorkspace) CentralConfigDir() (workspace.CentralConfigDir, error) { return f.dir, f.err }

func TestRunShowsResolvedConfigDir(t *testing.T) {
	ws := fakeWorkspace{dir: workspace.CentralConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}}

	out, err := run(load(t, "", nil, nil), ws)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^centralConfigDir\s+/work/config\s+cwd$`, out)
}

func TestRunShowsUnresolvedConfigDirAsUnset(t *testing.T) {
	out, err := run(load(t, "", nil, nil), fakeWorkspace{err: workspace.ErrNotFound})

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^centralConfigDir\s+\(none; \./config in a central checkout\)\s+default$`, out)
}

func TestRunReturnsOtherWorkspaceErrors(t *testing.T) {
	boom := errors.New("boom")

	_, err := run(load(t, "", nil, nil), fakeWorkspace{err: boom})

	assert.ErrorIs(t, err, boom)
}

func TestRunFollowsOutputSetting(t *testing.T) {
	s := load(t, "", nil, map[string]string{"output": "json"})
	ws := fakeWorkspace{dir: workspace.CentralConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}}

	out, err := run(s, ws)

	require.NoError(t, err)
	var got map[string]struct{ Value, Source string }
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, "/work/config", got["centralConfigDir"].Value)
}

func TestRunRejectsUnknownOutput(t *testing.T) {
	_, err := run(clicfg.Settings{Output: "xml"}, fakeWorkspace{})

	assert.Error(t, err)
}

func TestRunOpensWorkspaceInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "a.yaml"), []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: a\n"), 0o600))

	out, err := Run(load(t, "", nil, nil), dir)

	require.NoError(t, err)
	assert.Contains(t, out, filepath.Join(dir, "config"))
	assert.Regexp(t, `(?m)^centralConfigDir\s+\S+\s+cwd$`, out)
}

func TestRunWithoutCentralCheckout(t *testing.T) {
	out, err := Run(load(t, "", nil, nil), t.TempDir())

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^centralConfigDir\s+\(none; \./config in a central checkout\)\s+default$`, out)
}
