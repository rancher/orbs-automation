package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The schemas under generated/jsonschema are committed, so an editor can use them
// from a checkout. This keeps them from going stale when a type, a comment or a
// constant changes.
func TestTheCommittedSchemasAreWhatBuildMakes(t *testing.T) {
	t.Chdir("../..")
	want, err := build()
	require.NoError(t, err)

	dir := filepath.Join("generated", "jsonschema")
	got := map[string][]byte{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		got[filepath.ToSlash(rel)] = data
		return err
	})
	require.NoError(t, err, "run `go run ./tools/genschema` from the repository root")

	assert.ElementsMatch(t, keys(want), keys(got), "files that are missing or left over: run `go run ./tools/genschema`, and delete what it does not write")
	for name, data := range want {
		assert.Equal(t, string(data), string(got[name]), "%s is stale: run `go run ./tools/genschema`", name)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
