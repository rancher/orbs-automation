package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noGitHubAuth leaves no token anywhere go-gh looks: no environment variable,
// no gh config and no gh on PATH. go-gh reads gh's config once per process, so
// every test here points it at an empty directory before the first read.
func noGitHubAuth(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST", "GH_PATH",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("PATH", t.TempDir())
}

// resetHostname clears the local --hostname flag, which resetFlags (root's
// persistent flags only) does not know about.
func resetHostname(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cmd, _, err := rootCmd.Find([]string{"auth", "status"})
		require.NoError(t, err)
		f := cmd.Flags().Lookup("hostname")
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

// Wiring only: the logic is tested in internal/cli/authstatus. Nothing here
// reaches the network, because every case stops before a token exists.
func TestAuthStatusWiring(t *testing.T) {
	t.Run("without a token it fails and says what to set", func(t *testing.T) {
		noGitHubAuth(t)

		stdout, _, err := execRoot(t, "auth", "status")

		require.Error(t, err)
		assert.ErrorContains(t, err, "no GitHub token found for github.com")
		assert.ErrorContains(t, err, "GH_TOKEN")
		assert.Empty(t, stdout)
	})

	t.Run("--hostname reaches the check", func(t *testing.T) {
		noGitHubAuth(t)
		resetHostname(t)

		_, _, err := execRoot(t, "auth", "status", "--hostname", "ghe.example.com")

		require.Error(t, err)
		assert.ErrorContains(t, err, "no GitHub token found for ghe.example.com")
		assert.ErrorContains(t, err, "GH_ENTERPRISE_TOKEN")
	})

	t.Run("GH_HOST picks the default host", func(t *testing.T) {
		noGitHubAuth(t)
		t.Setenv("GH_HOST", "ghe.example.com")

		_, _, err := execRoot(t, "auth", "status")

		require.Error(t, err)
		assert.ErrorContains(t, err, "no GitHub token found for ghe.example.com")
	})

	t.Run("--output env is refused", func(t *testing.T) {
		noGitHubAuth(t)

		stdout, _, err := execRoot(t, "auth", "status", "--output", "env")

		require.Error(t, err)
		assert.ErrorContains(t, err, "does not support output format")
		assert.Empty(t, stdout)
	})

	t.Run("takes no arguments", func(t *testing.T) {
		noGitHubAuth(t)

		_, _, err := execRoot(t, "auth", "status", "extra")

		assert.Error(t, err)
	})
}
