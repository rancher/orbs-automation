package backport

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

// The helpers of proposaltest, under the short names the tests of this package use.
var (
	runGit = proposaltest.RunGit
	put    = proposaltest.Put
)

// landed is a repository on main with a first commit, and a way to land commits on it as
// Author.
func landed(t *testing.T) string {
	t.Helper()
	dir := proposaltest.Remote(t)
	runGit(t, dir, "config", "user.name", "Dev")
	runGit(t, dir, "config", "user.email", "dev@example.com")
	return dir
}

// commit makes a commit by Author on the branch checked out, and returns it.
func commit(t *testing.T, dir, file, message string) string {
	t.Helper()
	put(t, dir, file, message+"\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "--author=Author <author@example.com>", "-m", message)
	return runGit(t, dir, "rev-parse", "HEAD")
}

func prCommit(sha, message string) pulls.Commit {
	return pulls.Commit{SHA: sha, Message: message, Author: "Author"}
}

func TestChooseOfOneCommitPicksIt(t *testing.T) {
	dir := landed(t)
	m := commit(t, dir, "a.txt", "Fix the thing")

	got, err := Choose(bg, GitCLI{}, dir, PR{MergeCommit: m, Commits: []pulls.Commit{prCommit("x1", "Fix the thing")}})

	require.NoError(t, err)
	assert.Equal(t, Picks{Shape: ShapeSingle, Commits: []string{m}}, got)
}

func TestChooseOfARebaseMergePicksEachCommitOldestFirst(t *testing.T) {
	dir := landed(t)
	commit(t, dir, "early.txt", "Unrelated earlier")
	c1 := commit(t, dir, "a.txt", "First part")
	c2 := commit(t, dir, "b.txt", "Second part")
	pr := PR{MergeCommit: c2, Commits: []pulls.Commit{prCommit("x1", "First part"), prCommit("x2", "Second part")}}

	got, err := Choose(bg, GitCLI{}, dir, pr)

	require.NoError(t, err)
	assert.Equal(t, Picks{Shape: ShapeRebase, Commits: []string{c1, c2}}, got)
}

func TestChooseOfASquashPicksTheOneCommit(t *testing.T) {
	dir := landed(t)
	commit(t, dir, "early.txt", "Unrelated earlier")
	m := commit(t, dir, "a.txt", "Fix the thing (#12)")
	pr := PR{MergeCommit: m, Commits: []pulls.Commit{prCommit("x1", "wip"), prCommit("x2", "review fixes")}}

	got, err := Choose(bg, GitCLI{}, dir, pr)

	require.NoError(t, err)
	assert.Equal(t, Picks{Shape: ShapeSquash, Commits: []string{m}}, got)
}

func TestChooseOfARebaseNeedsTheAuthorsToMatchToo(t *testing.T) {
	dir := landed(t)
	commit(t, dir, "a.txt", "Same words")
	m := commit(t, dir, "b.txt", "More words")
	pr := PR{MergeCommit: m, Commits: []pulls.Commit{
		{SHA: "x1", Message: "Same words", Author: "Someone Else"}, {SHA: "x2", Message: "More words", Author: "Author"},
	}}

	got, err := Choose(bg, GitCLI{}, dir, pr)

	require.NoError(t, err)
	assert.Equal(t, ShapeSquash, got.Shape, "commits by other authors are not the PR's commits rewritten")
}

func TestChooseOfAMergeCommitPicksItAgainstTheFirstParent(t *testing.T) {
	dir := landed(t)
	runGit(t, dir, "checkout", "--quiet", "-b", "topic")
	commit(t, dir, "t1.txt", "Topic one")
	commit(t, dir, "t2.txt", "Topic two")
	runGit(t, dir, "checkout", "--quiet", "main")
	runGit(t, dir, "merge", "--quiet", "--no-ff", "-m", "Merge pull request #12", "topic")
	m := runGit(t, dir, "rev-parse", "HEAD")
	pr := PR{MergeCommit: m, Commits: []pulls.Commit{prCommit("x1", "Topic one"), prCommit("x2", "Topic two")}}

	got, err := Choose(bg, GitCLI{}, dir, pr)

	require.NoError(t, err)
	assert.Equal(t, Picks{Shape: ShapeMerge, Commits: []string{m}, Mainline: 1}, got)
}

func TestChooseRefusesWhatItCannotReadAndNamesIt(t *testing.T) {
	dir := landed(t)
	m := commit(t, dir, "a.txt", "Fix")
	one := []pulls.Commit{prCommit("x1", "Fix")}

	_, noMerge := Choose(bg, GitCLI{}, dir, PR{Number: 12, Commits: one})
	_, noCommits := Choose(bg, GitCLI{}, dir, PR{Number: 12, MergeCommit: m})
	_, unknown := Choose(bg, GitCLI{}, dir, PR{Number: 12, MergeCommit: "0123456789abcdef0123456789abcdef01234567", Commits: one})

	require.Error(t, noMerge)
	assert.Contains(t, noMerge.Error(), "#12")
	assert.Contains(t, noMerge.Error(), "merge commit")
	require.Error(t, noCommits)
	assert.Contains(t, noCommits.Error(), "commits")
	require.Error(t, unknown)
	assert.Contains(t, unknown.Error(), "0123456")
}
