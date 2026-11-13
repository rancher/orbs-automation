package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A run in CI owns its checkout, so a file an earlier step left in it is part of
// the work and goes in the commit; on a workstation the engineer's own untracked
// files are left alone.
func TestShipInCIStagesTheUntrackedFilesOfTheCheckout(t *testing.T) {
	ws, clone, ex := newShip(t)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "left-by-a-step.txt"), []byte("x\n"), 0o644))
	ex.Identify = identifying(Identity{Name: "bot[bot]", Email: "1+bot[bot]@users.noreply.github.com"}, nil)
	in := shipIn("v2.15")
	in.CI = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	require.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	assert.Contains(t, runGit(t, clone, "show", "--name-only", "--format=", botV215), "left-by-a-step.txt")
}

func TestShipOnAWorkstationLeavesTheUntrackedFilesOfTheClone(t *testing.T) {
	ws, clone, ex := newShip(t)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "notes.txt"), []byte("mine\n"), 0o644))

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	require.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	assert.NotContains(t, runGit(t, clone, "show", "--name-only", "--format=", botV215), "notes.txt")
}
