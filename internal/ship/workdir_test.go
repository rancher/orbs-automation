package ship

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v\n%s", args, out)
	return strings.TrimSpace(string(out))
}

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func slurp(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// remote is a consumer on the host: main holds build.yaml, and release/v1 adds
// app/gen.go.
func remote(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "--initial-branch=main")
	put(t, dir, "build.yaml", "image: old\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "first")
	runGit(t, dir, "checkout", "--quiet", "-b", "release/v1")
	put(t, dir, "app/gen.go", "package app\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "release")
	runGit(t, dir, "checkout", "--quiet", "main")
	return dir
}

// engineer is the engineer's clone of remote, with the remote named upstream.
func engineer(t *testing.T, up string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rancher")
	runGit(t, t.TempDir(), "clone", "--quiet", "--origin", "upstream", up, dir)
	runGit(t, dir, "config", "user.name", "Dev")
	runGit(t, dir, "config", "user.email", "dev@example.com")
	return dir
}

// request prepares release/v1 as the new branch bot/x-v1, from the commit the
// remote has for it now.
func request(t *testing.T, up, clone string) PrepareRequest {
	t.Helper()
	return PrepareRequest{
		CloneDir: clone, Remote: "upstream", Target: "release/v1",
		Base: runGit(t, up, "rev-parse", "release/v1"), Branch: "bot/x-v1", TempDir: t.TempDir(),
	}
}

func current(t *testing.T, dir string) string { return runGit(t, dir, "branch", "--show-current") }

func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	return runGit(t, dir, "branch", "--list", name) != ""
}

func TestPrepareWorksInPlaceInACleanClone(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)

	w, err := Prepare(bg, GitCLI{}, req)

	require.NoError(t, err)
	assert.Equal(t, InPlace, w.Place)
	assert.Empty(t, w.Reason)
	assert.Equal(t, clone, w.Dir)
	assert.Equal(t, "bot/x-v1", current(t, clone))
	assert.Equal(t, req.Base, runGit(t, clone, "rev-parse", "HEAD"), "at the commit that was checked")
	assert.FileExists(t, filepath.Join(clone, "app", "gen.go"))
	assert.Equal(t, runGit(t, up, "rev-parse", "main"), runGit(t, clone, "rev-parse", "main"), "no local branch was moved")
}

func TestPrepareUsesAWorktreeForADirtyCloneAndLeavesItAlone(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "build.yaml", "image: mine\n")
	req := request(t, up, clone)

	w, err := Prepare(bg, GitCLI{}, req)

	require.NoError(t, err)
	assert.Equal(t, Worktree, w.Place)
	assert.Contains(t, w.Reason, "uncommitted")
	assert.NotEqual(t, clone, w.Dir)
	assert.True(t, strings.HasPrefix(w.Dir, req.TempDir), "under the temp directory")
	assert.Equal(t, "bot/x-v1", current(t, w.Dir))
	assert.FileExists(t, filepath.Join(w.Dir, "app", "gen.go"))
	assert.Equal(t, "main", current(t, clone))
	assert.Equal(t, "image: mine\n", slurp(t, filepath.Join(clone, "build.yaml")), "the engineer's edit is untouched")
}

func TestPrepareDryRunAlwaysUsesAWorktreeAndFinishRemovesBoth(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	req.DryRun = true

	w := mustPrepare(t, req)
	assert.Equal(t, Worktree, w.Place)
	assert.Contains(t, w.Reason, "dry run")
	assert.Equal(t, "main", current(t, clone))

	require.NoError(t, w.Finish(bg, GitCLI{}, false))

	assert.NoDirExists(t, w.Dir)
	assert.False(t, branchExists(t, clone, "bot/x-v1"))
	assert.Equal(t, "main", current(t, clone))
}

func TestPrepareSendsAnUntrackedFileInTheWayToAWorktree(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "app/gen.go", "mine, not committed\n") // release/v1 has this path
	req := request(t, up, clone)

	w, err := Prepare(bg, GitCLI{}, req)

	require.NoError(t, err)
	assert.Equal(t, Worktree, w.Place)
	assert.Contains(t, w.Reason, "app/gen.go")
	assert.Equal(t, "mine, not committed\n", slurp(t, filepath.Join(clone, "app", "gen.go")))
}

func TestPrepareKeepsAnUntrackedFileThatIsNotInTheWayAndWorksInPlace(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "notes.txt", "mine")

	w, err := Prepare(bg, GitCLI{}, request(t, up, clone))

	require.NoError(t, err)
	assert.Equal(t, InPlace, w.Place)
	assert.Equal(t, "mine", slurp(t, filepath.Join(clone, "notes.txt")))
}

func TestPrepareFailsWhenTheTargetMovedSinceItWasChecked(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	runGit(t, up, "checkout", "--quiet", "release/v1")
	put(t, up, "later.txt", "x")
	runGit(t, up, "add", ".")
	runGit(t, up, "commit", "--quiet", "-m", "later")

	w, err := Prepare(bg, GitCLI{}, req)

	require.ErrorIs(t, err, ErrBaseMoved)
	assert.Nil(t, w)
	assert.False(t, branchExists(t, clone, "bot/x-v1"), "nothing was created")
	assert.Equal(t, "main", current(t, clone))
}

func TestPrepareFailsForATargetTheRemoteLacks(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	req.Target = "release/v9"

	w, err := Prepare(bg, GitCLI{}, req)

	require.Error(t, err)
	assert.Nil(t, w)
	assert.Contains(t, err.Error(), "release/v9")
}

func TestPrepareReplacesABranchThatIsIdenticalToTheBase(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	runGit(t, clone, "fetch", "--quiet", "upstream", "release/v1")
	runGit(t, clone, "branch", "bot/x-v1", req.Base)

	w, err := Prepare(bg, GitCLI{}, req)

	require.NoError(t, err)
	assert.Equal(t, "bot/x-v1", current(t, w.Dir))
	assert.Equal(t, req.Base, runGit(t, w.Dir, "rev-parse", "HEAD"))
}

func TestPrepareRefusesABranchWithCommitsOfItsOwn(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	runGit(t, clone, "fetch", "--quiet", "upstream", "release/v1")
	runGit(t, clone, "checkout", "--quiet", "-b", "bot/x-v1", req.Base)
	put(t, clone, "mine.txt", "x")
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "--quiet", "-m", "mine")
	runGit(t, clone, "checkout", "--quiet", "main")
	own := runGit(t, clone, "rev-parse", "bot/x-v1")

	w, err := Prepare(bg, GitCLI{}, req)

	require.ErrorIs(t, err, ErrBranchExists)
	assert.Nil(t, w)
	assert.Contains(t, err.Error(), "bot/x-v1")
	assert.Equal(t, own, runGit(t, clone, "rev-parse", "bot/x-v1"), "their commits are still there")
}

func TestPrepareRefusesABranchThatIsCheckedOut(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	req := request(t, up, clone)
	runGit(t, clone, "fetch", "--quiet", "upstream", "release/v1")
	runGit(t, clone, "checkout", "--quiet", "-b", "bot/x-v1", req.Base)

	w, err := Prepare(bg, GitCLI{}, req)

	require.ErrorIs(t, err, ErrBranchCheckedOut)
	assert.Nil(t, w)
}

func TestCommitInPlaceTakesTheEditAndTheCreatedFilesButNotTheEngineersOwn(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "notes.txt", "engineer's own")
	w := mustPrepare(t, request(t, up, clone))
	put(t, w.Dir, "build.yaml", "image: new\n")
	put(t, w.Dir, "generated/out.go", "package g\n")

	hash, err := w.Commit(bg, GitCLI{}, GitCommit{Message: "Update\n\nbody"})

	require.NoError(t, err)
	assert.Equal(t, hash, runGit(t, w.Dir, "rev-parse", "HEAD"))
	assert.ElementsMatch(t, []string{"build.yaml", "generated/out.go"},
		strings.Fields(runGit(t, w.Dir, "show", "--name-only", "--format=", "HEAD")))
	assert.Equal(t, "engineer's own", slurp(t, filepath.Join(clone, "notes.txt")))
	st, err := gitcli.Status(bg, clone)
	require.NoError(t, err)
	assert.Equal(t, []string{"notes.txt"}, st.Untracked)
}

func TestCommitInAWorktreeTakesEverythingTheRunCreated(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "build.yaml", "image: mine\n")
	w := mustPrepare(t, request(t, up, clone))
	require.Equal(t, Worktree, w.Place)
	put(t, w.Dir, "build.yaml", "image: new\n")
	put(t, w.Dir, "generated/out.go", "package g\n")

	_, err := w.Commit(bg, GitCLI{}, GitCommit{Message: "m"})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"build.yaml", "generated/out.go"},
		strings.Fields(runGit(t, w.Dir, "show", "--name-only", "--format=", "HEAD")))
}

func TestCommitWithNothingChangedIsNothingToCommit(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	w := mustPrepare(t, request(t, up, clone))

	_, err := w.Commit(bg, GitCLI{}, GitCommit{Message: "m"})

	require.ErrorIs(t, err, gitcli.ErrNothingToCommit)
}

func TestFinishInPlaceReturnsToTheStartingBranchAndKeepsOrDeletesTheNewOne(t *testing.T) {
	for _, keep := range []bool{true, false} {
		up := remote(t)
		clone := engineer(t, up)
		w := mustPrepare(t, request(t, up, clone))
		put(t, w.Dir, "build.yaml", "image: new\n")
		_, err := w.Commit(bg, GitCLI{}, GitCommit{Message: "m"})
		require.NoError(t, err)

		require.NoError(t, w.Finish(bg, GitCLI{}, keep))

		assert.Equal(t, "main", current(t, clone), "keep %v", keep)
		assert.Equal(t, keep, branchExists(t, clone, "bot/x-v1"), "keep %v", keep)
		assert.Equal(t, "image: old\n", slurp(t, filepath.Join(clone, "build.yaml")))
	}
}

func TestFinishAWorktreeRemovesItAndKeepsTheBranchWhenAsked(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "build.yaml", "image: mine\n")
	w := mustPrepare(t, request(t, up, clone))
	put(t, w.Dir, "build.yaml", "image: new\n")
	hash, err := w.Commit(bg, GitCLI{}, GitCommit{Message: "m"})
	require.NoError(t, err)

	require.NoError(t, w.Finish(bg, GitCLI{}, true))

	assert.NoDirExists(t, w.Dir)
	assert.Equal(t, hash, runGit(t, clone, "rev-parse", "bot/x-v1"), "the commit is there to review")
	assert.Equal(t, "image: mine\n", slurp(t, filepath.Join(clone, "build.yaml")))
}

func TestAbortInPlaceUndoesTheRunAndNothingOfTheEngineers(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "notes.txt", "engineer's own")
	w := mustPrepare(t, request(t, up, clone))
	put(t, w.Dir, "build.yaml", "image: half done\n")
	put(t, w.Dir, "generated/out.go", "package g\n")

	require.NoError(t, w.Abort(bg, GitCLI{}))

	assert.Equal(t, "main", current(t, clone))
	assert.False(t, branchExists(t, clone, "bot/x-v1"))
	assert.Equal(t, "image: old\n", slurp(t, filepath.Join(clone, "build.yaml")))
	assert.NoFileExists(t, filepath.Join(clone, "generated", "out.go"))
	assert.Equal(t, "engineer's own", slurp(t, filepath.Join(clone, "notes.txt")))
}

func TestAbortAWorktreeRemovesItAndTheBranch(t *testing.T) {
	up := remote(t)
	clone := engineer(t, up)
	put(t, clone, "build.yaml", "image: mine\n")
	w := mustPrepare(t, request(t, up, clone))
	put(t, w.Dir, "build.yaml", "image: half done\n")

	require.NoError(t, w.Abort(bg, GitCLI{}))

	assert.NoDirExists(t, w.Dir)
	assert.False(t, branchExists(t, clone, "bot/x-v1"))
	assert.Equal(t, "image: mine\n", slurp(t, filepath.Join(clone, "build.yaml")))
}

// mustPrepare prepares and requires a work directory that exists, so a test that
// edits in it cannot write into the package directory when Prepare does nothing.
func mustPrepare(t *testing.T, req PrepareRequest) *Work {
	t.Helper()
	w, err := Prepare(bg, GitCLI{}, req)
	require.NoError(t, err)
	require.NotNil(t, w)
	require.DirExists(t, w.Dir)
	return w
}
