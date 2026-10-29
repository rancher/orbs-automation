package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalProject = `apiVersion: gero/v1alpha1
kind: Project
name: a
spec:
  repo:
    name: o/a
`

// writeConfig writes files, by name, into a new directory and returns it.
func writeConfig(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	return dir
}

func TestConfigLoadsAndAppliesDefaults(t *testing.T) {
	dir := writeConfig(t, map[string]string{"a.project.yaml": minimalProject})
	s := settings(t, "", nil, map[string]string{"config-dir": dir})

	got, err := open(s, "/elsewhere", nil).Config()

	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	assert.Equal(t, "a", got.Projects[0].Name)
	assert.Equal(t, "main", got.Projects[0].Repo.DefaultBranch, "defaults are applied")
	assert.Equal(t, "single", got.Projects[0].Release.Branches)
}

func TestConfigFindsTheCentralCheckoutInTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "a.project.yaml"), []byte(minimalProject), 0o600))

	got, err := Open(settings(t, "", nil, nil), root).Config()

	require.NoError(t, err)
	assert.Len(t, got.Projects, 1)
}

func TestConfigWithoutAConfigDirectory(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).Config()

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestConfigLoadErrorNamesTheFile(t *testing.T) {
	dir := writeConfig(t, map[string]string{"bad.project.yaml": "kind: [unclosed\n"})
	s := settings(t, "", nil, map[string]string{"config-dir": dir})

	_, err := open(s, "/elsewhere", nil).Config()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad.project.yaml")
}

func TestConfigRejectsAnInvalidConfig(t *testing.T) {
	dir := writeConfig(t, map[string]string{"a.project.yaml": `apiVersion: gero/v1alpha1
kind: Project
name: a
spec:
  repo:
    name: not-a-slug
`})
	s := settings(t, "", nil, map[string]string{"config-dir": dir})

	_, err := open(s, "/elsewhere", nil).Config()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo.name")
}

func TestConfigLoadsTheCheckedInDirectories(t *testing.T) {
	for _, rel := range []string{"../../config", "../../config-mock"} {
		t.Run(rel, func(t *testing.T) {
			dir, err := filepath.Abs(rel)
			require.NoError(t, err)
			s := settings(t, "", nil, map[string]string{"config-dir": dir})

			got, err := open(s, "/elsewhere", nil).Config()

			require.NoError(t, err)
			assert.NotEmpty(t, got.Projects)
		})
	}
}
