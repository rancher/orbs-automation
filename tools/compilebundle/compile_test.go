package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtures is the design fixture, relative to the repository root: stable test state
// that does not move when the real config/ does.
const fixtures = "docs/design/fixture"

var header = bundle.Header{
	GeroVersion:  "v0.5.0",
	SourceCommit: "abc123",
	CompiledAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
}

func TestRunWritesTheBundleToTheOutputDirectory(t *testing.T) {
	t.Chdir("../..")
	out := filepath.Join(t.TempDir(), "dist", "nested")

	require.NoError(t, run(fixtures, out, header))

	got, err := os.ReadFile(filepath.Join(out, "gero-config-bundle.yaml"))
	require.NoError(t, err)
	want, err := bundle.Compile(os.DirFS(fixtures), header)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestRunWritesNothingForConfigThatDoesNotValidate(t *testing.T) {
	t.Chdir("../..")
	bad := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bad, "a.yaml"), []byte("a: [\n"), 0o644))
	out := filepath.Join(t.TempDir(), "dist")

	require.Error(t, run(bad, out, header))

	assert.NoDirExists(t, out)
}

func TestRunMustStartAtTheRepositoryRoot(t *testing.T) {
	t.Chdir(t.TempDir())

	err := run(fixtures, t.TempDir(), header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository root")
}
