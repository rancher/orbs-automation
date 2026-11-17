// Package proposaltest holds the helpers that the tests of every package that proposes a
// change share: real git repositories on disk, a holder on the host with the engineer's
// clone of it, and the shape of a PR as the host lists it. It imports no gero package, so
// the tests of internal/proposal can use it.
package proposaltest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/execx"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

// RunGit runs git in dir and returns what it printed, trimmed. A failure fails the test.
func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	env := append(execx.DefaultEnv(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	res, err := execx.New("git").Run(context.Background(), execx.Cmd{Name: "git", Args: append([]string{"-C", dir}, args...), Env: env})
	if err != nil {
		t.Fatalf("git %v: %v\n%s%s", args, err, res.Stdout, res.Stderr)
	}
	return strings.TrimSpace(string(res.Stdout) + string(res.Stderr))
}

// Put writes a file under dir, making its directories.
func Put(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Slurp reads a file.
func Slurp(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Remote is a holder on the host: main holds build.yaml, and release/v1 adds
// app/gen.go. The test is skipped when there is no git binary.
func Remote(t *testing.T) string {
	t.Helper()
	if _, err := execx.New("git").Lookup("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	RunGit(t, dir, "init", "--quiet", "--initial-branch=main")
	Put(t, dir, "build.yaml", "image: old\n")
	RunGit(t, dir, "add", ".")
	RunGit(t, dir, "commit", "--quiet", "-m", "first")
	RunGit(t, dir, "checkout", "--quiet", "-b", "release/v1")
	Put(t, dir, "app/gen.go", "package app\n")
	RunGit(t, dir, "add", ".")
	RunGit(t, dir, "commit", "--quiet", "-m", "release")
	RunGit(t, dir, "checkout", "--quiet", "main")
	return dir
}

// Engineer is the engineer's clone of a remote, with the remote named upstream.
func Engineer(t *testing.T, up string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rancher")
	RunGit(t, t.TempDir(), "clone", "--quiet", "--origin", "upstream", up, dir)
	RunGit(t, dir, "config", "user.name", "Dev")
	RunGit(t, dir, "config", "user.email", "dev@example.com")
	return dir
}

// Current is the branch a clone is on.
func Current(t *testing.T, dir string) string { return RunGit(t, dir, "branch", "--show-current") }

// PR is an open PR by author whose body is a summary and the given marker text.
func PR(number int, author, marker string) pulls.PR {
	return pulls.PR{Number: number, Author: author, Body: "## Summary\n\n" + marker}
}
