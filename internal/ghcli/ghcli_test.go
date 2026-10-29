package ghcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeGH writes an executable script called name into a new directory and
// returns the directory.
func writeGH(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755))
	return dir
}

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLocateFindsGhOnPath(t *testing.T) {
	dir := writeGH(t, "gh", "exit 0\n")
	t.Setenv("PATH", dir)

	path, err := Locate(env(nil))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "gh"), path)
}

func TestLocateNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	path, err := Locate(env(nil))

	require.ErrorIs(t, err, ErrNotFound)
	assert.EqualError(t, err, "gh not found on PATH")
	assert.Empty(t, path)
}

func TestLocateGHPathWinsOverPath(t *testing.T) {
	onPath := writeGH(t, "gh", "exit 0\n")
	other := writeGH(t, "mygh", "exit 0\n")
	t.Setenv("PATH", onPath)

	path, err := Locate(env(map[string]string{"GH_PATH": filepath.Join(other, "mygh")}))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(other, "mygh"), path)
}

func TestLocateGHPathThatDoesNotExist(t *testing.T) {
	onPath := writeGH(t, "gh", "exit 0\n")
	t.Setenv("PATH", onPath) // gh is on PATH, but go-gh would still run GH_PATH
	missing := filepath.Join(t.TempDir(), "nope")

	path, err := Locate(env(map[string]string{"GH_PATH": missing}))

	require.ErrorIs(t, err, ErrNotFound)
	assert.ErrorContains(t, err, "GH_PATH")
	assert.ErrorContains(t, err, missing)
	assert.Empty(t, path)
}

func TestVersionIsTheFirstLine(t *testing.T) {
	dir := writeGH(t, "gh", "echo 'gh version 2.100.0 (2026-09-03)'\necho 'https://github.com/cli/cli/releases/tag/v2.100.0'\n")

	v, err := Version(context.Background(), filepath.Join(dir, "gh"))

	require.NoError(t, err)
	assert.Equal(t, "gh version 2.100.0 (2026-09-03)", v)
}

func TestVersionPassesTheVersionFlag(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	dir := writeGH(t, "gh", "echo \"$@\" > "+argsFile+"\necho 'gh version 1'\n")

	_, err := Version(context.Background(), filepath.Join(dir, "gh"))

	require.NoError(t, err)
	got, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	assert.Equal(t, "--version\n", string(got))
}

func TestVersionFailure(t *testing.T) {
	dir := writeGH(t, "gh", "echo 'boom' >&2\nexit 1\n")

	v, err := Version(context.Background(), filepath.Join(dir, "gh"))

	require.Error(t, err)
	assert.Empty(t, v)
}

func TestVersionWithNoOutputIsAnError(t *testing.T) {
	dir := writeGH(t, "gh", "exit 0\n")

	v, err := Version(context.Background(), filepath.Join(dir, "gh"))

	require.Error(t, err)
	assert.Empty(t, v)
}

func TestVersionStopsWhenTheContextEnds(t *testing.T) {
	dir := writeGH(t, "gh", "exec sleep 10\n") // exec, so killing gh ends the pipe
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Version(ctx, filepath.Join(dir, "gh"))

	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}
