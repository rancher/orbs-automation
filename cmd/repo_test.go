package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wiring only: the logic is tested in internal/cli/branchtags.
func TestRepoBranchTagsWiring(t *testing.T) {
	t.Chdir(t.TempDir()) // not inside any repository, so no tags

	t.Run("CI inputs", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/heads/release/v2.x")
		t.Setenv("GITHUB_SHA", "abcdef1234567890")

		stdout, _, err := execRoot(t, "repo", "branch-tags")

		require.NoError(t, err)
		assert.Equal(t, "branch_tag=v2.x-head\nbranch_static_tag=v2.x-head-abcdef1\nprev_static_tag=\n", stdout)
	})

	t.Run("unsupported ref: message on stdout only, error returned", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/pull/12/merge")
		t.Setenv("GITHUB_SHA", "abcdef1")

		stdout, stderr, err := execRoot(t, "repo", "branch-tags")

		assert.Error(t, err)
		assert.Contains(t, stdout, "unsupported branch pattern")
		assert.Empty(t, stderr)
	})

	t.Run("not a repository", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "")
		t.Setenv("GITHUB_SHA", "")

		_, _, err := execRoot(t, "repo", "branch-tags")

		assert.Error(t, err)
	})
}
