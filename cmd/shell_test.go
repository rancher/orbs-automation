package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wiring only: the logic is tested in internal/shellenv and internal/cli/shellsetup.
func TestShellInitPrintsCompletionAndTheWrapper(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		stdout, _, err := execRoot(t, "shell", "init", shell)

		require.NoError(t, err, shell)
		assert.Contains(t, stdout, "_gero", shell+": the cobra completion")
		assert.Contains(t, stdout, "gero", shell)
		assert.Contains(t, stdout, "cd", shell)
	}
}

func TestShellInitRejectsAShellWeDoNotSupport(t *testing.T) {
	_, _, err := execRoot(t, "shell", "init", "tcsh")

	require.Error(t, err)
}

func TestShellInstallAndUninstallEditTheNamedProfile(t *testing.T) {
	profile := filepath.Join(t.TempDir(), ".zshrc")

	_, _, err := execRoot(t, "shell", "install", "--shell", "zsh", "--profile", profile, "--yes")
	require.NoError(t, err)
	data, err := os.ReadFile(profile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "gero shell init zsh")

	_, _, err = execRoot(t, "shell", "uninstall", "--shell", "zsh", "--profile", profile, "--yes")
	require.NoError(t, err)
	data, err = os.ReadFile(profile)
	require.NoError(t, err)
	assert.Empty(t, string(data))
}

func TestShellInstallDryRunWritesNothing(t *testing.T) {
	profile := filepath.Join(t.TempDir(), ".zshrc")

	stdout, _, err := execRoot(t, "--dry-run", "shell", "install", "--shell", "zsh", "--profile", profile)

	require.NoError(t, err)
	assert.Contains(t, stdout, "nothing was written")
	_, statErr := os.Stat(profile)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
