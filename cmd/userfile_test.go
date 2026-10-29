package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/doctor"
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

// The user-file check is where a broken file gets reported, since doctor, unlike
// every other command, runs without it.
func TestDoctorReportsTheUserFile(t *testing.T) {
	t.Run("a broken file fails the check and says how to fix it", func(t *testing.T) {
		resetDoctorFlags(t)
		path := brokenUserFile(t)

		stdout, _, err := execRoot(t, "doctor", "user-file")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ user-file: ")
		assert.Contains(t, stdout, path)
		assert.Contains(t, stdout, "bogus")
		assert.Contains(t, stdout, "→ fix what the message names, or move the file aside")
	})

	t.Run("a good file is reported by its path", func(t *testing.T) {
		resetDoctorFlags(t)
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("output: text\n"), 0o600))
		t.Setenv("GERO_CONFIG", path)

		stdout, _, err := execRoot(t, "doctor", "user-file")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ user-file: "+path+"\n")
	})

	t.Run("no file is fine", func(t *testing.T) {
		resetDoctorFlags(t)
		t.Setenv("GERO_CONFIG", "") // execRoot points XDG_CONFIG_HOME at an empty directory

		stdout, _, err := execRoot(t, "doctor", "user-file")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ user-file: no user file at ")
		assert.Contains(t, stdout, "(optional)")
	})
}
