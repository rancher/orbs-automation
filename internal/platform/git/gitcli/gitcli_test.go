package gitcli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v\n%s", args, out)
	return strings.TrimSpace(string(out))
}

func TestShortCommit(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "--quiet")
	run(t, dir, "commit", "--quiet", "--allow-empty", "-m", "first")
	full := run(t, dir, "rev-parse", "HEAD")

	got, err := ShortCommit(dir)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 7)
	assert.True(t, strings.HasPrefix(full, got), "%q is not a prefix of %s", got, full)
	assert.Equal(t, run(t, dir, "rev-parse", "--short", "HEAD"), got, "must match git itself")
}

func TestShortCommitNoCommits(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "--quiet")

	_, err := ShortCommit(dir)

	assert.Error(t, err)
}
