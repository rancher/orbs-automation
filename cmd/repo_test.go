package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wiring only: the logic is tested in internal/cli/branchtags.
func TestRepoBuildTagsWiring(t *testing.T) {
	t.Chdir(t.TempDir()) // not inside any repository, so no tags

	t.Run("CI inputs", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/heads/release/v2.x")
		t.Setenv("GITHUB_SHA", "abcdef1234567890")

		stdout, _, err := execRoot(t, "repo", "build-tags")

		require.NoError(t, err)
		assert.Equal(t, "branch_tag=v2.x-head\nbranch_static_tag=v2.x-head-abcdef1\nprev_static_tag=\n", stdout)
	})

	t.Run("unsupported ref: message on stdout only, error returned", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/pull/12/merge")
		t.Setenv("GITHUB_SHA", "abcdef1")

		stdout, stderr, err := execRoot(t, "repo", "build-tags")

		assert.Error(t, err)
		assert.Contains(t, stdout, "unsupported branch pattern")
		assert.Empty(t, stderr)
	})

	t.Run("not a repository", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "")
		t.Setenv("GITHUB_SHA", "")

		_, _, err := execRoot(t, "repo", "build-tags")

		assert.Error(t, err)
	})
}

// The command was called current-tags first, and a workflow may still say so.
func TestRepoBuildTagsIsStillReachableAsCurrentTags(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_SHA", "abcdef1234567890")

	stdout, _, err := execRoot(t, "repo", "current-tags")

	require.NoError(t, err)
	assert.Equal(t, "branch_tag=head\nbranch_static_tag=main-abcdef1\nprev_static_tag=\n", stdout)
}

func TestRepoCurrentTagsIsNotListedAmongTheCommands(t *testing.T) {
	stdout, _, err := execRoot(t, "repo", "--help")

	require.NoError(t, err)
	assert.Contains(t, stdout, "build-tags")
	assert.NotContains(t, stdout, "current-tags", "an old name is an alias, not a second command in the list")
}

// Wiring only: the logic is tested in internal/cli/repobranch.
func TestRepoBranchWiring(t *testing.T) {
	t.Run("takes a line and a name, with a project first when there is one", func(t *testing.T) {
		_, _, err := execRoot(t, "repo", "branch", "v2.15")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 and 3")

		_, _, err = execRoot(t, "repo", "branch", "a", "b", "c", "d")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 and 3")
	})

	t.Run("outside any project it is the project that is missing, and nothing is printed", func(t *testing.T) {
		t.Chdir(t.TempDir())

		stdout, _, err := execRoot(t, "repo", "branch", "v2.15", "my-fix")

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "unknown command")
		assert.Empty(t, stdout)
	})

	// Last: cobra keeps the help flag set on the command between runs.
	t.Run("the help says what it does and has --switch", func(t *testing.T) {
		stdout, _, err := execRoot(t, "repo", "branch", "--help")

		require.NoError(t, err)
		assert.Contains(t, stdout, "--switch")
		assert.Contains(t, stdout, "release line")
	})
}
