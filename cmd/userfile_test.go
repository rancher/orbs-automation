package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokenUserFile writes a user file with a key that does not exist, which the
// loader refuses, and points GERO_CONFIG at it.
func brokenUserFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("bogus: 1\n"), 0o600))
	t.Setenv("GERO_CONFIG", path)
	return path
}

// An ordinary command stops on a user file it cannot load, and on a setting it
// cannot accept. Only gero doctor relaxes the first, so it can report it.
func TestOrdinaryCommandsStopOnABrokenUserFile(t *testing.T) {
	t.Run("an unknown key", func(t *testing.T) {
		path := brokenUserFile(t)

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

// gero doctor has to run to report a user file it cannot load, so it carries on
// without the file. Everything else it is given still has to be valid.
func TestDoctorRunsDespiteABrokenUserFile(t *testing.T) {
	t.Run("an unknown key", func(t *testing.T) {
		resetDoctorFlags(t)
		brokenUserFile(t)

		stdout, _, err := execRoot(t, "doctor", "version")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ version")
	})

	t.Run("a GERO_CONFIG that names a missing file", func(t *testing.T) {
		resetDoctorFlags(t)
		t.Setenv("GERO_CONFIG", filepath.Join(t.TempDir(), "nope.yaml"))

		stdout, _, err := execRoot(t, "doctor", "version")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ version")
	})

	t.Run("the environment and flags still apply without the file", func(t *testing.T) {
		resetDoctorFlags(t)
		brokenUserFile(t)
		t.Setenv("GERO_OUTPUT", "json")

		stdout, _, err := execRoot(t, "doctor", "version")

		require.NoError(t, err)
		assert.Contains(t, stdout, `"checks": [`)
	})

	t.Run("an environment value that does not exist still stops it", func(t *testing.T) {
		resetDoctorFlags(t)
		brokenUserFile(t)
		t.Setenv("GERO_OUTPUT", "bogus")

		stdout, _, err := execRoot(t, "doctor", "version")

		require.Error(t, err, "the file was excused, the bad environment value was not")
		assert.Empty(t, stdout)
	})
}
