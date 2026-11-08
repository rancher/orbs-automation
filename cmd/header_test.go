package cmd

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/header"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetHeaderFlags clears --egg, which resetFlags (root's persistent flags
// only) does not know about.
func resetHeaderFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"version", "doctor"} {
			cmd, _, err := rootCmd.Find([]string{name})
			require.NoError(t, err)
			if f := cmd.Flags().Lookup("egg"); f != nil {
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			}
		}
		resetDoctorFlags(t)
	})
}

func TestVersionHeaderPrintsTheArtAfterTheVersion(t *testing.T) {
	resetHeaderFlags(t)

	stdout, _, err := execRoot(t, "version", "--egg")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n"+headerTitle+"\n"+strings.TrimRight(header.Print(), "\n")+"\n", stdout)
}

func TestVersionWithoutTheHeaderFlagPrintsNoArt(t *testing.T) {
	stdout, _, err := execRoot(t, "version")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n", stdout)
}

func TestHeaderLeavesStructuredOutputAlone(t *testing.T) {
	resetHeaderFlags(t)

	stdout, _, err := execRoot(t, "version", "--egg", "--json")

	require.NoError(t, err)
	assert.NotContains(t, stdout, "█")
	assert.JSONEq(t, `{"version":"dev","commit":"none","date":"unknown"}`, stdout)
}

func TestHeaderLeavesEnvOutputAlone(t *testing.T) {
	resetHeaderFlags(t)

	stdout, _, err := execRoot(t, "doctor", "--egg", "--list-checks", "--output", "env")

	require.Error(t, err, "doctor has no env format")
	assert.NotContains(t, stdout, "█")
}

func TestDoctorHeaderPrintsTheArtBeforeTheReport(t *testing.T) {
	resetHeaderFlags(t)

	stdout, _, err := execRoot(t, "doctor", "--egg", "--list-checks")

	require.NoError(t, err)
	art := headerTitle + "\n" + strings.TrimRight(header.Print(), "\n") + "\n"
	assert.True(t, strings.HasPrefix(stdout, art), "the art ends its last line, so the report starts on a new one")
	assert.Greater(t, len(stdout), len(art), "the report follows the art")
}

func TestHeaderIsHiddenFromHelp(t *testing.T) {
	for _, name := range []string{"version", "doctor"} {
		t.Run(name, func(t *testing.T) {
			// --help stays set on the shared command after a run.
			t.Cleanup(func() {
				cmd, _, err := rootCmd.Find([]string{name})
				require.NoError(t, err)
				f := cmd.Flags().Lookup("help")
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			})
			stdout, _, err := execRoot(t, name, "--help")

			require.NoError(t, err)
			assert.NotContains(t, stdout, "--egg")
		})
	}
}
