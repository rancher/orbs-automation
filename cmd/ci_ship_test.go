package cmd

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/cigate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ci:ship takes what ship takes where a workflow can use it, and nothing that
// asks, skips the push, or chooses where to push: it is never interactive and
// pushes to the upstream.
func TestCIShipFlagsFollowShip(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"ci:ship"})
	require.NoError(t, err)
	require.Equal(t, "ci:ship", c.Name())

	for _, name := range []string{"to", "line", "prerelease", "allow-downgrade", "preview", "refresh"} {
		assert.NotNil(t, c.Flags().Lookup(name), "--%s", name)
	}
	for _, name := range []string{"yes", "non-interactive", "no-push", "push-to"} {
		assert.Nil(t, c.Flags().Lookup(name), "--%s", name)
	}
}

func TestCIShipNeedsCIContextAndToken(t *testing.T) {
	for _, k := range []string{"GITHUB_ACTIONS", "CI", "GERO_CONTEXT"} {
		t.Setenv(k, "")
	}
	old := cigate.Auth
	cigate.Auth = stubAuth{token: "t0ken"}
	t.Cleanup(func() { cigate.Auth = old })

	_, _, err := execRoot(t, "ci:ship")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CI context")
}

func TestCIShipRefusesTheWorkflowsOwnToken(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "t0ken")
	t.Setenv("GH_TOKEN", "")
	old := cigate.Auth
	cigate.Auth = stubAuth{token: "t0ken"}
	t.Cleanup(func() { cigate.Auth = old })

	_, _, err := execRoot(t, "ci:ship")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GH_TOKEN")
}
