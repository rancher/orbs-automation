package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGit puts a git script first and alone on PATH and returns the file it
// records its arguments in.
func fakeGit(t *testing.T, script string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	body := "#!/bin/sh\necho \"$@\" > " + argsFile + "\n" + script
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0o755))
	t.Setenv("PATH", dir)
	return argsFile
}

func noGit(t *testing.T) { t.Helper(); t.Setenv("PATH", t.TempDir()) }

func TestVersionIsTheFirstLine(t *testing.T) {
	args := fakeGit(t, "echo 'git version 2.51.0'\necho 'extra line'\n")

	v, err := Version(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "git version 2.51.0", v)
	got, err := os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "--version\n", string(got))
}

func TestVersionWithoutGit(t *testing.T) {
	noGit(t)

	v, err := Version(context.Background())

	require.ErrorIs(t, err, ErrNotFound)
	assert.Empty(t, v)
}

func TestVersionFailureIsNotNotFound(t *testing.T) {
	fakeGit(t, "echo boom >&2\nexit 1\n")

	v, err := Version(context.Background())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.Empty(t, v)
}

func TestVersionWithNoOutputIsAnError(t *testing.T) {
	fakeGit(t, "exit 0\n")

	_, err := Version(context.Background())

	require.Error(t, err)
}

func TestVersionStopsWhenTheContextEnds(t *testing.T) {
	fakeGit(t, "exec sleep 10\n") // exec, so killing git ends the pipe
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Version(ctx)

	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestConfigReturnsTheValue(t *testing.T) {
	args := fakeGit(t, "echo 'Ada Lovelace'\n")

	v, err := Config(context.Background(), "user.name")

	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", v)
	got, err := os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "config --get user.name\n", string(got))
}

func TestConfigUnsetIsEmptyNotAnError(t *testing.T) {
	fakeGit(t, "exit 1\n") // what git config --get does for a key that is not set

	v, err := Config(context.Background(), "user.email")

	require.NoError(t, err)
	assert.Empty(t, v)
}

func TestConfigOtherFailureIsAnError(t *testing.T) {
	fakeGit(t, "echo 'fatal: bad config' >&2\nexit 128\n")

	_, err := Config(context.Background(), "user.name")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestConfigWithoutGit(t *testing.T) {
	noGit(t)

	_, err := Config(context.Background(), "user.name")

	require.ErrorIs(t, err, ErrNotFound)
}
