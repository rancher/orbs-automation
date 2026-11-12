package bundle

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goldenFile = "testdata/golden-bundle.yaml"

// TestCompileMakesTheBytesTheGoldenFileHolds is what keeps "the same files, the same
// bundle" true when Compile changes. If it fails, Compile now makes different bytes
// for the same files: raise CompilerRevision in bundle.go, then write the golden file
// again with UPDATE_GOLDEN=1 go test ./internal/bundle -run Golden. A store keys a
// bundle by the digest and the revision, so a change that skips the raise would let
// a stored bundle stand for one Compile would no longer make.
func TestCompileMakesTheBytesTheGoldenFileHolds(t *testing.T) {
	got, err := Compile(fixture(), header)
	require.NoError(t, err)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(goldenFile, got, 0o644))
	}

	want, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got), "raise CompilerRevision, then UPDATE_GOLDEN=1 go test ./internal/bundle -run Golden")
}
