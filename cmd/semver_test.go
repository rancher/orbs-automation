package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wiring only: the logic is tested in internal/cli/checksemver.
func TestSemverCheckWiring(t *testing.T) {
	t.Setenv("GITHUB_REF_NAME", "")

	t.Run("argument, extra arguments ignored", func(t *testing.T) {
		stdout, _, err := execRoot(t, "semver", "check", "v1.2.3-rc.1", "ignored")

		require.NoError(t, err)
		assert.Equal(t, "HAS_PRERELEASE=true\nHAS_BUILD_META=false\n", stdout)
	})

	t.Run("GITHUB_REF_NAME when no argument", func(t *testing.T) {
		t.Setenv("GITHUB_REF_NAME", "v2.0.0+build.5")

		stdout, _, err := execRoot(t, "semver", "check")

		require.NoError(t, err)
		assert.Equal(t, "HAS_PRERELEASE=false\nHAS_BUILD_META=true\n", stdout)
	})

	t.Run("no tag is an error with empty stdout", func(t *testing.T) {
		stdout, _, err := execRoot(t, "semver", "check")

		assert.Error(t, err)
		assert.Empty(t, stdout)
	})
}

// check-semver classifies a string and never opens a repository, so it is not
// a `repo` command.
func TestCheckSemverIsNotUnderRepo(t *testing.T) {
	stdout, _, _ := execRoot(t, "repo", "check-semver", "v1.2.3")

	assert.NotContains(t, stdout, "HAS_PRERELEASE")
}
