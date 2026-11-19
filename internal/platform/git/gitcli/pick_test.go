package gitcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pickRepo is a repository on release/v1, which has a.txt and diverged from main, where
// three commits by Author were made on top of the same first commit: edit changes a line of
// a.txt, which release/v1 changed too; add adds b.txt, which merges cleanly. The release
// branch is checked out.
type pickShas struct{ edit, add string }

func pickRepo(t *testing.T) (string, pickShas) {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "--quiet", "--initial-branch=main")
	run(t, dir, "config", "user.name", "Dev")
	run(t, dir, "config", "user.email", "dev@example.com")
	write(t, dir, "a.txt", "1\n2\n3\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "first")
	run(t, dir, "branch", "release/v1")
	write(t, dir, "a.txt", "1\ntwo\n3\n")
	run(t, dir, "commit", "--quiet", "-a", "--author=Author <author@example.com>", "-m", "Edit line two")
	var s pickShas
	s.edit = run(t, dir, "rev-parse", "HEAD")
	write(t, dir, "b.txt", "b\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "--author=Author <author@example.com>", "-m", "Add b")
	s.add = run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "checkout", "--quiet", "release/v1")
	write(t, dir, "a.txt", "1\ndeux\n3\n")
	run(t, dir, "commit", "--quiet", "-a", "-m", "Release edit")
	return dir, s
}

func TestCherryPickKeepsTheAuthorRecordsTheOriginAndCommitsAsTheActor(t *testing.T) {
	dir, s := pickRepo(t)

	got, err := CherryPick(ctx, dir, Pick{Commit: s.add, Name: "Bot", Email: "bot@example.com"})

	require.NoError(t, err)
	assert.Equal(t, run(t, dir, "rev-parse", "HEAD"), got)
	assert.Equal(t, "Author", run(t, dir, "log", "-1", "--format=%an"))
	assert.Equal(t, "Bot", run(t, dir, "log", "-1", "--format=%cn"))
	assert.Contains(t, run(t, dir, "log", "-1", "--format=%B"), "(cherry picked from commit "+s.add+")")
	assert.FileExists(t, filepath.Join(dir, "b.txt"))
}

func TestCherryPickOfAMergeTakesItAgainstTheMainline(t *testing.T) {
	dir, s := pickRepo(t)
	run(t, dir, "checkout", "--quiet", "main")
	run(t, dir, "checkout", "--quiet", "-b", "topic", s.edit+"~1")
	write(t, dir, "c.txt", "c\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "Topic")
	run(t, dir, "checkout", "--quiet", "main")
	run(t, dir, "merge", "--quiet", "--no-ff", "-m", "Merge topic", "topic")
	merge := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "checkout", "--quiet", "release/v1")

	_, err := CherryPick(ctx, dir, Pick{Commit: merge, Mainline: 1})

	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "c.txt"))
	assert.Equal(t, "1", run(t, dir, "rev-list", "--count", "HEAD~1..HEAD"), "one commit, not the merged history")
}

func TestCherryPickThatConflictsLeavesThePickInProgressAndNamesTheFiles(t *testing.T) {
	dir, s := pickRepo(t)
	before := run(t, dir, "rev-parse", "HEAD")

	_, err := CherryPick(ctx, dir, Pick{Commit: s.edit})

	var conflict *ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, []string{"a.txt"}, conflict.Files)
	assert.Equal(t, before, run(t, dir, "rev-parse", "HEAD"), "nothing was committed")
	left, err := Unmerged(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt"}, left)
	assert.Equal(t, "cherry-pick", mustStatus(t, dir).InProgress)
}

func mustStatus(t *testing.T, dir string) State {
	t.Helper()
	st, err := Status(ctx, dir)
	require.NoError(t, err)
	return st
}

func TestConflictMarkersListsTheFilesThatStillHoldOne(t *testing.T) {
	dir, s := pickRepo(t)
	_, err := CherryPick(ctx, dir, Pick{Commit: s.edit})
	require.Error(t, err)

	left, err := ConflictMarkers(dir, []string{"a.txt"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt"}, left, "the file has markers as git left it")

	write(t, dir, "a.txt", "1\nresolved\n3\n")
	left, err = ConflictMarkers(dir, []string{"a.txt"})
	require.NoError(t, err)
	assert.Empty(t, left)
}

func TestContinuePickFinishesAResolvedPickWithItsMessage(t *testing.T) {
	dir, s := pickRepo(t)
	_, err := CherryPick(ctx, dir, Pick{Commit: s.edit})
	require.Error(t, err)
	write(t, dir, "a.txt", "1\nresolved\n3\n")

	got, err := ContinuePick(ctx, dir, []string{"a.txt"}, Pick{Name: "Bot", Email: "bot@example.com"})

	require.NoError(t, err)
	assert.Equal(t, run(t, dir, "rev-parse", "HEAD"), got)
	message := run(t, dir, "log", "-1", "--format=%B")
	assert.Contains(t, message, "Edit line two")
	assert.Contains(t, message, "(cherry picked from commit "+s.edit+")")
	assert.Equal(t, "Author", run(t, dir, "log", "-1", "--format=%an"))
	assert.Equal(t, "Bot", run(t, dir, "log", "-1", "--format=%cn"))
	assert.Empty(t, mustStatus(t, dir).InProgress)
	assert.Equal(t, "1\nresolved\n3\n", readFile(t, dir, "a.txt"))
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	require.NoError(t, err)
	return string(b)
}

func TestAbortPickPutsTheTreeBack(t *testing.T) {
	dir, s := pickRepo(t)
	before := run(t, dir, "rev-parse", "HEAD")
	_, err := CherryPick(ctx, dir, Pick{Commit: s.edit})
	require.Error(t, err)

	require.NoError(t, AbortPick(ctx, dir))

	assert.Equal(t, before, run(t, dir, "rev-parse", "HEAD"))
	assert.Empty(t, mustStatus(t, dir).InProgress)
	assert.Equal(t, "1\ndeux\n3\n", readFile(t, dir, "a.txt"))
}

func TestParentCountSeparatesAMergeFromAPlainCommit(t *testing.T) {
	dir, s := pickRepo(t)
	run(t, dir, "checkout", "--quiet", "main")
	run(t, dir, "checkout", "--quiet", "-b", "topic", s.edit+"~1")
	write(t, dir, "c.txt", "c\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "Topic")
	run(t, dir, "checkout", "--quiet", "main")
	run(t, dir, "merge", "--quiet", "--no-ff", "-m", "Merge topic", "topic")

	plain, err := ParentCount(ctx, dir, s.add)
	require.NoError(t, err)
	merge, err := ParentCount(ctx, dir, "main")
	require.NoError(t, err)

	assert.Equal(t, 1, plain)
	assert.Equal(t, 2, merge)
}

func TestFirstParentLogListsTheCommitsEndingAtTipOldestFirst(t *testing.T) {
	dir, s := pickRepo(t)

	got, err := FirstParentLog(ctx, dir, s.add, 2)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, s.edit, got[0].Hash)
	assert.Equal(t, "Edit line two", got[0].Message)
	assert.Equal(t, "Author", got[0].Author)
	assert.Equal(t, s.add, got[1].Hash)
	assert.Equal(t, "Add b", got[1].Message)
}

func TestHoldsPickOfFindsAnEarlierPickByItsOriginLine(t *testing.T) {
	dir, s := pickRepo(t)
	has, err := HoldsPickOf(ctx, dir, "release/v1", s.add)
	require.NoError(t, err)
	assert.False(t, has, "not picked yet")

	_, err = CherryPick(ctx, dir, Pick{Commit: s.add})
	require.NoError(t, err)
	has, err = HoldsPickOf(ctx, dir, "release/v1", s.add)
	require.NoError(t, err)
	assert.True(t, has)
	other, err := HoldsPickOf(ctx, dir, "release/v1", s.edit)
	require.NoError(t, err)
	assert.False(t, other, "another commit is not found")
}
