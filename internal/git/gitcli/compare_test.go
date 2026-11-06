package gitcli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// branchOff returns a clone on branch name that has made one commit changing
// build.yaml to image, started from the current head of main.
func branchOff(t *testing.T, dir, name, image string) string {
	t.Helper()
	run(t, dir, "checkout", "--quiet", "-B", name, "main")
	write(t, dir, "build.yaml", "image: "+image+"\n")
	run(t, dir, "commit", "--quiet", "-am", "bump to "+image+"\n\nAutomation: gero ship")
	return run(t, dir, "rev-parse", "HEAD")
}

func TestMergeBaseIsWhereTheBranchLeftTheOther(t *testing.T) {
	dir := clone(t, upstream(t))
	base := run(t, dir, "rev-parse", "main")
	tip := branchOff(t, dir, "bot/x", "new")
	run(t, dir, "checkout", "--quiet", "main")
	write(t, dir, "other.txt", "main moved on")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "main moved")

	got, err := MergeBase(ctx, dir, "main", tip)

	require.NoError(t, err)
	assert.Equal(t, base, got)
}

func TestMergeBaseOfUnrelatedCommitsIsAnError(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "checkout", "--quiet", "--orphan", "island")
	run(t, dir, "commit", "--quiet", "--allow-empty", "-m", "unrelated")

	_, err := MergeBase(ctx, dir, "main", "island")

	require.Error(t, err)
}

func TestCommitsSinceListsWhatABranchAddedOldestFirstWithWhoMadeThem(t *testing.T) {
	dir := clone(t, upstream(t))
	base := run(t, dir, "rev-parse", "main")
	run(t, dir, "checkout", "--quiet", "-b", "bot/x")
	write(t, dir, "build.yaml", "image: new\n")
	run(t, dir, "commit", "--quiet", "-am", "first\n\nbody line\n\nAutomation: gero ship")
	first := run(t, dir, "rev-parse", "HEAD")
	write(t, dir, "generated/out.go", "package g\n")
	run(t, dir, "add", ".")
	run(t, dir, "-c", "user.email=other@example.com", "commit", "--quiet", "--author=Someone <other@example.com>", "-m", "second")
	second := run(t, dir, "rev-parse", "HEAD")

	got, err := CommitsSince(ctx, dir, base, "bot/x")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, first, got[0].Hash)
	assert.Equal(t, "t@example.com", got[0].AuthorEmail, "the author the test helper commits as")
	assert.Contains(t, got[0].Message, "body line")
	assert.Contains(t, got[0].Message, "Automation: gero ship", "the trailers are in the message")
	assert.Equal(t, second, got[1].Hash)
	assert.Equal(t, "other@example.com", got[1].AuthorEmail)
}

func TestCommitsSinceNothingIsEmpty(t *testing.T) {
	dir := clone(t, upstream(t))

	got, err := CommitsSince(ctx, dir, "main", "main")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestChangeFingerprintIsTheSameForTheSameChangeOnADifferentBase(t *testing.T) {
	dir := clone(t, upstream(t))
	oldBase := run(t, dir, "rev-parse", "main")
	oldTip := branchOff(t, dir, "bot/old", "new")
	run(t, dir, "checkout", "--quiet", "main")
	write(t, dir, "other.txt", "main moved on\n")
	write(t, dir, "build.yaml", "# a comment above\nimage: old\n") // moves the line the change is on
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "main moved")
	newBase := run(t, dir, "rev-parse", "main")
	run(t, dir, "checkout", "--quiet", "-b", "bot/new")
	write(t, dir, "build.yaml", "# a comment above\nimage: new\n")
	run(t, dir, "commit", "--quiet", "-am", "bump again")
	newTip := run(t, dir, "rev-parse", "HEAD")

	before, err := ChangeFingerprint(ctx, dir, oldBase, oldTip)
	require.NoError(t, err)
	after, err := ChangeFingerprint(ctx, dir, newBase, newTip)
	require.NoError(t, err)

	require.NotEmpty(t, before)
	assert.Equal(t, before, after, "the same change to the same line, though the file and the base around it moved")
}

func TestChangeFingerprintDiffersForADifferentChange(t *testing.T) {
	dir := clone(t, upstream(t))
	base := run(t, dir, "rev-parse", "main")
	a := branchOff(t, dir, "bot/a", "one")
	b := branchOff(t, dir, "bot/b", "two")

	fa, err := ChangeFingerprint(ctx, dir, base, a)
	require.NoError(t, err)
	fb, err := ChangeFingerprint(ctx, dir, base, b)
	require.NoError(t, err)

	require.NotEmpty(t, fa)
	assert.NotEqual(t, fa, fb)
}

func TestChangeFingerprintSeesAnAddedAndADeletedFile(t *testing.T) {
	dir := clone(t, upstream(t))
	base := run(t, dir, "rev-parse", "main")
	only := branchOff(t, dir, "bot/only", "one")
	run(t, dir, "checkout", "--quiet", "-B", "bot/more", "main")
	write(t, dir, "build.yaml", "image: one\n")
	write(t, dir, "generated/out.go", "package g\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "--quiet", "-m", "more")
	more := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "checkout", "--quiet", "-B", "bot/less", "main")
	run(t, dir, "rm", "--quiet", "build.yaml")
	run(t, dir, "commit", "--quiet", "-m", "less")
	less := run(t, dir, "rev-parse", "HEAD")

	f := func(tip string) string {
		got, err := ChangeFingerprint(ctx, dir, base, tip)
		require.NoError(t, err)
		return got
	}

	assert.NotEqual(t, f(only), f(more), "an added file is part of the change")
	assert.NotEqual(t, f(only), f(less), "a deleted file is part of the change")
}

func TestChangeFingerprintOfNoChangeIsStableAndNotEmpty(t *testing.T) {
	dir := clone(t, upstream(t))

	a, err := ChangeFingerprint(ctx, dir, "main", "main")
	require.NoError(t, err)
	b, err := ChangeFingerprint(ctx, dir, "main", "main")
	require.NoError(t, err)

	assert.Equal(t, a, b)
	assert.NotEmpty(t, a)
	assert.Len(t, a, 64, "a sha-256 in hex")
}

func TestConfigInReadsTheClonesSettingsAndEmptyWhenUnset(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "user.email", "bot@example.com")

	got, err := ConfigIn(ctx, dir, "user.email")
	require.NoError(t, err)
	assert.Equal(t, "bot@example.com", got)

	none, err := ConfigIn(ctx, dir, "gero.nothing")
	require.NoError(t, err)
	assert.Empty(t, none)
	assert.False(t, strings.Contains(none, "\n"))
}
