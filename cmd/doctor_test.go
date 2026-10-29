package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/cli/doctor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetDoctorFlags clears the local flags of gero doctor, which resetFlags
// (root's persistent flags only) does not know about.
func resetDoctorFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cmd, _, err := rootCmd.Find([]string{"doctor"})
		require.NoError(t, err)
		for _, name := range []string{"online", "timeout", "strict", "list-checks"} {
			f := cmd.Flags().Lookup(name)
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
}

// stubGh puts a fake gh first on PATH, so the gh check does not depend on the
// machine the tests run on.
func stubGh(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'gh version 9.9.9 (2030-01-01)'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("GH_PATH", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Wiring only: the logic is tested in internal/cli/doctor.
func TestDoctorWiring(t *testing.T) {
	t.Run("runs the checks offline and prints the report", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err)
		assert.Contains(t, stdout, "Environment\n  ✔ version: gero dev on ")
		assert.Contains(t, stdout, "Dependencies\n  ✔ gh: gh version 9.9.9 (2030-01-01) at ")
		assert.Contains(t, stdout, "Summary: ")
		assert.Contains(t, stdout, "Your workstation is ready to use gero.")
	})

	t.Run("a named check runs alone", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "version")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ version")
		assert.NotContains(t, stdout, "gh:")
	})

	t.Run("an unknown check is an error that points at the list", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "nope")

		require.Error(t, err)
		assert.ErrorContains(t, err, `unknown check "nope"`)
		assert.ErrorContains(t, err, "--list-checks")
		assert.Empty(t, stdout)
	})

	t.Run("--list-checks lists them without running", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "--list-checks")

		require.NoError(t, err)
		assert.Contains(t, stdout, "NAME")
		assert.Contains(t, stdout, "version")
		assert.Contains(t, stdout, "gh")
		assert.NotContains(t, stdout, "Summary:")
	})

	t.Run("--json prints the report as json", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor", "--json")

		require.NoError(t, err)
		assert.Contains(t, stdout, `"checks": [`)
		assert.Contains(t, stdout, `"name": "version"`)
	})

	t.Run("a missing gh is a warning, not a failure", func(t *testing.T) {
		resetDoctorFlags(t)
		t.Setenv("GH_PATH", "")
		t.Setenv("PATH", t.TempDir())

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err, "a warning does not fail the run")
		assert.Contains(t, stdout, "⚠ gh: gh not found on PATH\n")
		assert.Contains(t, stdout, "Ready, with warnings.")
	})

	t.Run("--strict fails the run on that warning", func(t *testing.T) {
		resetDoctorFlags(t)
		t.Setenv("GH_PATH", "")
		t.Setenv("PATH", t.TempDir())

		stdout, _, err := execRoot(t, "doctor", "--strict")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "⚠ gh")
	})

	t.Run("--output env is refused", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "--output", "env")

		require.Error(t, err)
		assert.ErrorContains(t, err, "does not support output format")
		assert.Empty(t, stdout)
	})

	t.Run("flags are declared with their defaults", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"doctor"})
		require.NoError(t, err)

		d, err := cmd.Flags().GetDuration("timeout")
		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, d)
		for _, name := range []string{"online", "strict", "list-checks"} {
			b, err := cmd.Flags().GetBool(name)
			require.NoError(t, err)
			assert.False(t, b, name)
		}
	})
}
