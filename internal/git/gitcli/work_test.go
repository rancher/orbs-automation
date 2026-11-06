package gitcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// upstream makes a repository with a main branch holding build.yaml and a
// release/v1 branch that adds app/gen.go, and returns its path.
func upstream(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "--quiet", "--initial-branch=main")
	write(t, dir, "build.yaml", "image: old\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "first")
	run(t, dir, "checkout", "--quiet", "-b", "release/v1")
	write(t, dir, "app/gen.go", "package app\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "release")
	run(t, dir, "checkout", "--quiet", "main")
	return dir
}

// clone clones up, with up as the remote named upstream, on main, and returns it.
func clone(t *testing.T, up string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "work")
	run(t, t.TempDir(), "clone", "--quiet", "--origin", "upstream", up, dir)
	run(t, dir, "config", "user.name", "Dev")
	run(t, dir, "config", "user.email", "dev@example.com")
	return dir
}

func TestFetchReturnsTheBranchCommitAndMovesNoLocalBranch(t *testing.T) {
	up := upstream(t)
	dir := clone(t, up)
	run(t, up, "checkout", "--quiet", "release/v1")
	write(t, up, "new.txt", "n")
	run(t, up, "add", ".")
	run(t, up, "commit", "--quiet", "-m", "later")
	want := run(t, up, "rev-parse", "HEAD")
	mainBefore := run(t, dir, "rev-parse", "main")

	got, err := Fetch(ctx, dir, "upstream", "release/v1")

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, mainBefore, run(t, dir, "rev-parse", "main"))
	_, found, err := BranchCommit(ctx, dir, "release/v1")
	require.NoError(t, err)
	assert.False(t, found, "no local branch is created")
	has, err := HasCommit(ctx, dir, want)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestFetchOfAMissingBranchFails(t *testing.T) {
	dir := clone(t, upstream(t))

	_, err := Fetch(ctx, dir, "upstream", "release/v9")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "release/v9")
}

func TestHasCommitAndBranchCommit(t *testing.T) {
	dir := clone(t, upstream(t))
	head := run(t, dir, "rev-parse", "HEAD")

	has, err := HasCommit(ctx, dir, head)
	require.NoError(t, err)
	assert.True(t, has)
	has, err = HasCommit(ctx, dir, strings.Repeat("a", 40))
	require.NoError(t, err)
	assert.False(t, has)

	got, found, err := BranchCommit(ctx, dir, "main")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, head, got)
	_, found, err = BranchCommit(ctx, dir, "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestStatusOfACleanClone(t *testing.T) {
	st, err := Status(ctx, clone(t, upstream(t)))

	require.NoError(t, err)
	assert.Equal(t, State{Branch: "main"}, st)
	assert.True(t, st.Clean())
}

func TestStatusSeparatesTrackedChangesFromUntrackedFiles(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "build.yaml", "image: edited\n")
	write(t, dir, "notes/todo.txt", "mine")

	st, err := Status(ctx, dir)

	require.NoError(t, err)
	assert.Equal(t, []string{"build.yaml"}, st.Tracked)
	assert.Equal(t, []string{"notes/todo.txt"}, st.Untracked, "a file at a time, not the directory")
	assert.False(t, st.Clean())
}

func TestStatusUntrackedFilesAloneKeepItClean(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "scratch.txt", "mine")

	st, err := Status(ctx, dir)

	require.NoError(t, err)
	assert.True(t, st.Clean())
	assert.Equal(t, []string{"scratch.txt"}, st.Untracked)
}

func TestStatusStagedAndDeletedAndRenamedAreTracked(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "mv", "build.yaml", "renamed.yaml")

	st, err := Status(ctx, dir)

	require.NoError(t, err)
	assert.Equal(t, []string{"renamed.yaml"}, st.Tracked, "the old path of a rename is not a path of its own")
	assert.False(t, st.Clean())
}

func TestStatusDetachedHeadIsNotClean(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "checkout", "--quiet", "--detach")

	st, err := Status(ctx, dir)

	require.NoError(t, err)
	assert.Empty(t, st.Branch)
	assert.False(t, st.Clean())
}

func TestStatusNamesAnOperationInProgress(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "checkout", "--quiet", "-b", "side")
	write(t, dir, "build.yaml", "image: side\n")
	run(t, dir, "commit", "--quiet", "-am", "side")
	run(t, dir, "checkout", "--quiet", "main")
	write(t, dir, "build.yaml", "image: main\n")
	run(t, dir, "commit", "--quiet", "-am", "main")
	cmd := execGit(dir, "merge", "side")
	_ = cmd.Run() // conflicts, which is the point

	st, err := Status(ctx, dir)

	require.NoError(t, err)
	assert.Equal(t, "merge", st.InProgress)
	assert.False(t, st.Clean())
}

func TestCollisions(t *testing.T) {
	dir := clone(t, upstream(t))
	head := run(t, dir, "rev-parse", "upstream/release/v1")
	got, err := Collisions(ctx, dir, head, []string{
		"app/gen.go",    // the commit has this file
		"app/extra.txt", // its parent directory is fine: only gen.go is in the tree
		"notes/a.txt",   // nothing like it
		"build.yaml/x",  // the commit has a file where a directory would be
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"app/gen.go", "build.yaml/x"}, got)

	none, err := Collisions(ctx, dir, head, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestStartBranchCreatesItAtTheCommitAndCheckoutReturns(t *testing.T) {
	dir := clone(t, upstream(t))
	base, err := Fetch(ctx, dir, "upstream", "release/v1")
	require.NoError(t, err)

	require.NoError(t, StartBranch(ctx, dir, "bot/x-v1-v1", base))

	assert.Equal(t, "bot/x-v1-v1", run(t, dir, "branch", "--show-current"))
	assert.Equal(t, base, run(t, dir, "rev-parse", "HEAD"))
	assert.FileExists(t, filepath.Join(dir, "app", "gen.go"), "the working tree is the commit's")
	require.Error(t, StartBranch(ctx, dir, "bot/x-v1-v1", base), "an existing branch is an error")

	require.NoError(t, Checkout(ctx, dir, "main"))
	assert.Equal(t, "main", run(t, dir, "branch", "--show-current"))
	require.NoError(t, DeleteBranch(ctx, dir, "bot/x-v1-v1"))
	_, found, err := BranchCommit(ctx, dir, "bot/x-v1-v1")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestWorktreeLeavesTheCheckoutAlone(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "build.yaml", "image: mine\n") // uncommitted work
	base, err := Fetch(ctx, dir, "upstream", "release/v1")
	require.NoError(t, err)
	wt := filepath.Join(t.TempDir(), "wt")

	require.NoError(t, AddWorktree(ctx, dir, wt, "bot/x-v1-v1", base))

	assert.FileExists(t, filepath.Join(wt, "app", "gen.go"))
	got, err := os.ReadFile(filepath.Join(dir, "build.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "image: mine\n", string(got), "the engineer's edit is untouched")
	assert.Equal(t, "main", run(t, dir, "branch", "--show-current"))

	write(t, wt, "build.yaml", "image: bot\n")
	require.NoError(t, RemoveWorktree(ctx, dir, wt), "removed even with changes in it")
	_, err = os.Stat(wt)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCommitChangesStagesTrackedChangesAndTheNamedNewFilesOnly(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "build.yaml", "image: new\n")
	write(t, dir, "generated/out.go", "package g\n")
	write(t, dir, "mine.txt", "engineer's own file")

	hash, err := CommitChanges(ctx, dir, Commit{Message: "Update\n\nbody\n\nAutomation: x", Stage: []string{"generated/out.go"}})

	require.NoError(t, err)
	assert.Equal(t, hash, run(t, dir, "rev-parse", "HEAD"))
	files := run(t, dir, "show", "--name-only", "--format=", "HEAD")
	assert.ElementsMatch(t, []string{"build.yaml", "generated/out.go"}, strings.Fields(files))
	assert.Equal(t, "Update\n\nbody\n\nAutomation: x", run(t, dir, "log", "-1", "--format=%B"))
	assert.Equal(t, "Dev", run(t, dir, "log", "-1", "--format=%an"), "the clone's own git config")
	st, err := Status(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"mine.txt"}, st.Untracked, "the engineer's file is still theirs")
}

func TestCommitChangesWithAnIdentity(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "build.yaml", "image: new\n")

	_, err := CommitChanges(ctx, dir, Commit{Message: "m", Name: "gero", Email: "gero@example.com"})

	require.NoError(t, err)
	assert.Equal(t, "gero <gero@example.com>", run(t, dir, "log", "-1", "--format=%an <%ae>"))
	assert.Equal(t, "gero <gero@example.com>", run(t, dir, "log", "-1", "--format=%cn <%ce>"))
}

func TestCommitChangesIsUnsignedUnlessAsked(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "commit.gpgsign", "true")
	run(t, dir, "config", "gpg.program", "false") // signing would fail
	write(t, dir, "build.yaml", "image: new\n")

	_, err := CommitChanges(ctx, dir, Commit{Message: "m"})
	require.NoError(t, err, "the setting does not make gero sign")

	write(t, dir, "build.yaml", "image: newer\n")
	_, err = CommitChanges(ctx, dir, Commit{Message: "m", Sign: true})
	require.Error(t, err, "asked to sign, and signing fails, so the commit fails")
}

func TestCommitChangesWithNothingStagedIsAnError(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "mine.txt", "not ours")

	_, err := CommitChanges(ctx, dir, Commit{Message: "m"})

	require.ErrorIs(t, err, ErrNothingToCommit)
}

func TestCommitChangesRunsTheEngineersHooks(t *testing.T) {
	dir := clone(t, upstream(t))
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\necho blocked >&2\nexit 1\n"), 0o755))
	write(t, dir, "build.yaml", "image: new\n")

	_, err := CommitChanges(ctx, dir, Commit{Message: "m"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestDiscardUndoesOnlyTheRun(t *testing.T) {
	dir := clone(t, upstream(t))
	write(t, dir, "mine.txt", "engineer's file")
	write(t, dir, "build.yaml", "image: edited\n")
	write(t, dir, "generated/out.go", "package g\n")

	require.NoError(t, Discard(ctx, dir, []string{"generated/out.go"}))

	got, err := os.ReadFile(filepath.Join(dir, "build.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "image: old\n", string(got))
	assert.NoFileExists(t, filepath.Join(dir, "generated", "out.go"))
	assert.FileExists(t, filepath.Join(dir, "mine.txt"))
}

// execGit is git in dir with an identity, for a command whose failure the test
// expects.
func execGit(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	return cmd
}
