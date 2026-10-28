package gitcli

import (
	"os/exec"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestShortCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	run(t, dir, "init", "--quiet")
	run(t, dir, "commit", "--quiet", "--allow-empty", "-m", "first")
	full := run(t, dir, "rev-parse", "HEAD")

	got, err := ShortCommit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 7 || !strings.HasPrefix(full, got) {
		t.Errorf("ShortCommit = %q, want a prefix (7+ chars) of %s", got, full)
	}
	if want := run(t, dir, "rev-parse", "--short", "HEAD"); got != want {
		t.Errorf("ShortCommit = %q, git says %q", got, want)
	}
}

func TestShortCommitNoCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	run(t, dir, "init", "--quiet")
	if _, err := ShortCommit(dir); err == nil {
		t.Error("want an error with no commits")
	}
}
