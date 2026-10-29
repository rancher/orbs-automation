package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userFileWith writes content to a user file and points GERO_CONFIG at it.
func userFileWith(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	t.Setenv("GERO_CONFIG", path)
	return path
}

// An ordinary command stops on a user file it cannot load, and on a setting it
// cannot accept. Only gero doctor relaxes the first, so it can report it.
func TestOrdinaryCommandsStopOnABrokenUserFile(t *testing.T) {
	t.Run("an unknown key", func(t *testing.T) {
		path := userFileWith(t, "bogus: 1\n")

		stdout, _, err := execRoot(t, "semver", "check", "v1.2.3")

		require.Error(t, err)
		assert.ErrorContains(t, err, path)
		assert.ErrorContains(t, err, "bogus")
		assert.Empty(t, stdout)
	})

	t.Run("a GERO_CONFIG that names a missing file", func(t *testing.T) {
		t.Setenv("GERO_CONFIG", filepath.Join(t.TempDir(), "nope.yaml"))

		_, _, err := execRoot(t, "semver", "check", "v1.2.3")

		require.Error(t, err)
	})

	t.Run("an output format that does not exist", func(t *testing.T) {
		t.Setenv("GERO_OUTPUT", "bogus")

		_, _, err := execRoot(t, "semver", "check", "v1.2.3")

		require.Error(t, err)
	})
}
