package gitcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forkOf is a clone whose remote "fork" is a bare repository, with a commit on
// the branch bot/x that is not on the remote.
func forkOf(t *testing.T) (dir, bare string) {
	t.Helper()
	dir = clone(t, upstream(t))
	bare = filepath.Join(t.TempDir(), "fork.git")
	run(t, t.TempDir(), "init", "--quiet", "--bare", "--initial-branch=main", bare)
	run(t, dir, "remote", "add", "fork", bare)
	run(t, dir, "checkout", "--quiet", "-b", "bot/x")
	write(t, dir, "build.yaml", "image: new\n")
	run(t, dir, "commit", "--quiet", "-am", "bump")
	return dir, bare
}

func TestPushSendsTheBranchAndMakesItTheUpstream(t *testing.T) {
	dir, bare := forkOf(t)

	require.NoError(t, Push(ctx, dir, "fork", "bot/x"))

	assert.Equal(t, run(t, dir, "rev-parse", "HEAD"), run(t, bare, "rev-parse", "refs/heads/bot/x"))
	assert.Equal(t, "fork", run(t, dir, "config", "branch.bot/x.remote"))
	assert.Equal(t, "refs/heads/bot/x", run(t, dir, "config", "branch.bot/x.merge"), "the script's -u")
}

func TestPushAFurtherCommitIsAFastForward(t *testing.T) {
	dir, bare := forkOf(t)
	require.NoError(t, Push(ctx, dir, "fork", "bot/x"))
	write(t, dir, "generated/out.go", "package g\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "generated")

	require.NoError(t, Push(ctx, dir, "fork", "bot/x"))

	assert.Equal(t, run(t, dir, "rev-parse", "HEAD"), run(t, bare, "rev-parse", "refs/heads/bot/x"))
}

func TestPushIsNeverForced(t *testing.T) {
	dir, bare := forkOf(t)
	// The remote already has this branch, with a commit that the clone does not.
	other := clone(t, upstream(t))
	run(t, other, "checkout", "--quiet", "-b", "bot/x")
	write(t, other, "other.txt", "theirs")
	run(t, other, "add", ".")
	run(t, other, "commit", "--quiet", "-m", "someone else's")
	run(t, other, "push", "--quiet", bare, "bot/x")
	theirs := run(t, bare, "rev-parse", "refs/heads/bot/x")

	err := Push(ctx, dir, "fork", "bot/x")

	require.ErrorIs(t, err, ErrPushRejected)
	assert.Equal(t, theirs, run(t, bare, "rev-parse", "refs/heads/bot/x"), "the remote's commits are still there")
	assert.Contains(t, err.Error(), "rejected", "git's own words are kept")
}

func TestPushToARemoteThatDoesNotExistNamesIt(t *testing.T) {
	dir, _ := forkOf(t)

	err := Push(ctx, dir, "nope", "bot/x")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrPushRejected)
	assert.Contains(t, err.Error(), "nope")
}

func TestPushOfABranchThatDoesNotExistIsAnError(t *testing.T) {
	dir, _ := forkOf(t)

	err := Push(ctx, dir, "fork", "bot/missing")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bot/missing")
}

// ssh is a stand-in for the ssh command that records the environment it got and
// then fails, so what reached git's child can be seen without a network.
func ssh(t *testing.T) (command, envFile string) {
	t.Helper()
	dir := t.TempDir()
	envFile = filepath.Join(dir, "env.txt")
	command = filepath.Join(dir, "ssh")
	require.NoError(t, os.WriteFile(command, []byte("#!/bin/sh\nenv > '"+envFile+"'\nexit 1\n"), 0o755))
	return command, envFile
}

func TestPushReachesTheEngineersCredentialsAndNothingSecret(t *testing.T) {
	dir, _ := forkOf(t)
	run(t, dir, "remote", "add", "origin2", "ssh://git@example.invalid/x.git")
	command, envFile := ssh(t)
	t.Setenv("GIT_SSH_COMMAND", command)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/the-agent.sock")
	t.Setenv("GH_TOKEN", "secret-token")
	t.Setenv("GITHUB_TOKEN", "another-secret")

	err := Push(ctx, dir, "origin2", "bot/x")

	require.Error(t, err, "the stand-in ssh fails")
	data, readErr := os.ReadFile(envFile)
	require.NoError(t, readErr, "git ran the engineer's ssh command")
	env := string(data)
	assert.Contains(t, env, "SSH_AUTH_SOCK=/tmp/the-agent.sock", "the agent is reachable")
	assert.NotContains(t, env, "secret-token")
	assert.NotContains(t, env, "another-secret")
	assert.True(t, strings.Contains(env, "GIT_TERMINAL_PROMPT=0"), "git may not prompt on a terminal")
}
