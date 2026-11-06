package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wiring only: the logic is tested in internal/cli/cdpath.
func TestCdWiring(t *testing.T) {
	t.Chdir(t.TempDir())

	t.Run("a project is required", func(t *testing.T) {
		_, _, err := execRoot(t, "cd")

		require.Error(t, err)
	})

	t.Run("the implicit project is refused", func(t *testing.T) {
		stdout, _, err := execRoot(t, "cd", ".")

		require.Error(t, err)
		assert.Empty(t, stdout)
	})
}
