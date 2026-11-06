package gitcli

import (
	"os"
	"os/exec"
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

// TestPushSpikeToARealRemote pushes a throwaway branch to a real remote, through
// whatever credentials this machine has, and deletes it again. It settles what no
// local test can: that the variables Push passes on reach the engineer's SSH
// agent or credential helper. It is skipped unless GERO_SPIKE_PUSH_URL names a
// repository that is safe to push a branch to, such as a scratch fork:
//
//	GERO_SPIKE_PUSH_URL=git@github.com:you/scratch.git go test ./internal/git/gitcli -run PushSpike -v
//
// An https URL needs a credential helper (gh auth setup-git), since prompts are
// off.
func TestPushSpikeToARealRemote(t *testing.T) {
	url := os.Getenv("GERO_SPIKE_PUSH_URL")
	if url == "" {
		t.Skip("set GERO_SPIKE_PUSH_URL to a scratch repository to run the push spike")
	}
	requireGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "--quiet", "--initial-branch=main")
	run(t, dir, "config", "user.name", "gero spike")
	run(t, dir, "config", "user.email", "spike@example.invalid")
	write(t, dir, "spike.txt", "gero push spike\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "gero push spike")
	run(t, dir, "remote", "add", "spike", url)
	branch := "gero-spike-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + filepath.Base(dir)
	run(t, dir, "checkout", "--quiet", "-b", branch)
	t.Cleanup(func() { // best effort, with the full environment: a leftover branch is harmless but untidy
		_ = exec.Command("git", "-C", dir, "push", "--quiet", "spike", "--delete", branch).Run()
	})

	require.NoError(t, Push(ctx, dir, "spike", branch))

	remote := run(t, dir, "ls-remote", "--heads", "spike", branch)
	assert.Contains(t, remote, "refs/heads/"+branch, "the branch is on the real remote")
	assert.Contains(t, remote, run(t, dir, "rev-parse", "HEAD"))
}

// pushedFork is forkOf with the branch bot/x already pushed, and the commit that
// the remote has for it.
func pushedFork(t *testing.T) (dir, bare, old string) {
	t.Helper()
	dir, bare = forkOf(t)
	require.NoError(t, Push(ctx, dir, "fork", "bot/x"))
	return dir, bare, run(t, bare, "rev-parse", "refs/heads/bot/x")
}

// rebuilt replaces the branch's commit with one that has the same parent and a
// different change, as a rebuild does.
func rebuilt(t *testing.T, dir string) string {
	t.Helper()
	run(t, dir, "reset", "--quiet", "--hard", "HEAD~1")
	write(t, dir, "build.yaml", "image: rebuilt\n")
	run(t, dir, "commit", "--quiet", "-am", "bump again")
	return run(t, dir, "rev-parse", "HEAD")
}

func TestPushReplacingReplacesTheBranchWhenItIsWhereItWasSeen(t *testing.T) {
	dir, bare, old := pushedFork(t)
	fresh := rebuilt(t, dir)
	require.Error(t, Push(ctx, dir, "fork", "bot/x"), "a plain push is rejected: the history changed")

	err := PushReplacing(ctx, dir, "fork", "bot/x", old)

	require.NoError(t, err)
	assert.Equal(t, fresh, run(t, bare, "rev-parse", "refs/heads/bot/x"))
}

func TestPushReplacingIsRejectedWhenTheRemoteMovedSinceItWasSeen(t *testing.T) {
	dir, bare, old := pushedFork(t)
	// Someone else pushes to the branch after it was seen.
	other := clone(t, upstream(t))
	run(t, other, "fetch", "--quiet", bare, "bot/x")
	run(t, other, "checkout", "--quiet", "-b", "bot/x", "FETCH_HEAD")
	write(t, other, "theirs.txt", "x")
	run(t, other, "add", ".")
	run(t, other, "commit", "--quiet", "-m", "theirs")
	run(t, other, "push", "--quiet", bare, "bot/x")
	theirs := run(t, bare, "rev-parse", "refs/heads/bot/x")
	require.NotEqual(t, old, theirs)
	rebuilt(t, dir)

	err := PushReplacing(ctx, dir, "fork", "bot/x", old)

	require.ErrorIs(t, err, ErrPushRejected)
	assert.Equal(t, theirs, run(t, bare, "rev-parse", "refs/heads/bot/x"), "their commit is still there")
}

func TestPushReplacingABranchThatIsNotThereIsRejectedNotCreated(t *testing.T) {
	dir, bare := forkOf(t)

	err := PushReplacing(ctx, dir, "fork", "bot/x", strings.Repeat("a", 40))

	require.Error(t, err, "there is nothing at the commit that was seen")
	assert.Empty(t, run(t, bare, "branch", "--list", "bot/x"))
}

func TestPushReplacingUsesTheSameEnvironmentAsPush(t *testing.T) {
	dir, _ := forkOf(t)
	run(t, dir, "remote", "add", "origin2", "ssh://git@example.invalid/x.git")
	command, envFile := ssh(t)
	t.Setenv("GIT_SSH_COMMAND", command)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/the-agent.sock")
	t.Setenv("GH_TOKEN", "secret-token")

	err := PushReplacing(ctx, dir, "origin2", "bot/x", strings.Repeat("a", 40))

	require.Error(t, err)
	data, readErr := os.ReadFile(envFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "SSH_AUTH_SOCK=/tmp/the-agent.sock")
	assert.NotContains(t, string(data), "secret-token")
	assert.Contains(t, string(data), "GIT_TERMINAL_PROMPT=0")
}
