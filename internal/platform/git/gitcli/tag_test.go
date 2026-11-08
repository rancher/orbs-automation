package gitcli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTagMakesALightweightTagAtTheCommitAndMovesNothing(t *testing.T) {
	dir := clone(t, upstream(t))
	main := run(t, dir, "rev-parse", "HEAD")
	release := run(t, dir, "rev-parse", "upstream/release/v1")

	require.NoError(t, CreateTag(ctx, dir, "v1.0.0-rc.1", release))

	assert.Equal(t, release, run(t, dir, "rev-parse", "refs/tags/v1.0.0-rc.1"))
	assert.Equal(t, "commit", run(t, dir, "cat-file", "-t", "refs/tags/v1.0.0-rc.1"), "a ref and no tag object")
	assert.Equal(t, main, run(t, dir, "rev-parse", "HEAD"), "the checkout is where it was")
	assert.Equal(t, "main", run(t, dir, "branch", "--show-current"))
}

func TestCreateTagNeverMovesATagThatExists(t *testing.T) {
	dir := clone(t, upstream(t))
	first := run(t, dir, "rev-parse", "HEAD")
	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", first))

	err := CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "upstream/release/v1"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, first, run(t, dir, "rev-parse", "refs/tags/v1.0.0"))
}

func TestCreateTagAtACommitTheCloneLacksIsAnError(t *testing.T) {
	dir := clone(t, upstream(t))

	err := CreateTag(ctx, dir, "v1.0.0", "0123456789abcdef0123456789abcdef01234567")

	require.Error(t, err)
	assert.Empty(t, run(t, dir, "tag", "--list"))
}

func TestTagCommit(t *testing.T) {
	dir := clone(t, upstream(t))
	head := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "tag", "light", head)
	release := run(t, dir, "rev-parse", "upstream/release/v1")
	run(t, dir, "tag", "-a", "-m", "annotated", "heavy", release)

	got, found, err := TagCommit(ctx, dir, "light")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, head, got)

	got, found, err = TagCommit(ctx, dir, "heavy")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, release, got, "the commit it peels to, not the tag object")

	got, found, err = TagCommit(ctx, dir, "absent")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, got)
}

func TestPushTagSendsTheTag(t *testing.T) {
	dir, bare := forkOf(t)
	head := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "tag", "v1.0.0", head)

	require.NoError(t, PushTag(ctx, dir, "fork", "v1.0.0"))

	assert.Equal(t, head, run(t, bare, "rev-parse", "refs/tags/v1.0.0"))
	assert.Empty(t, run(t, bare, "branch", "--list"), "only the tag, no branch")
}

func TestPushTagOfATagTheRemoteHasOnTheSameCommitIsNothingToDo(t *testing.T) {
	dir, _ := forkOf(t)
	run(t, dir, "tag", "v1.0.0", run(t, dir, "rev-parse", "HEAD"))
	require.NoError(t, PushTag(ctx, dir, "fork", "v1.0.0"))

	require.NoError(t, PushTag(ctx, dir, "fork", "v1.0.0"))
}

func TestPushTagIsNeverForcedOverATagOnAnotherCommit(t *testing.T) {
	dir, bare := forkOf(t)
	head := run(t, dir, "rev-parse", "HEAD")
	other := run(t, dir, "rev-parse", "upstream/release/v1")
	run(t, dir, "tag", "v1.0.0", other)
	require.NoError(t, PushTag(ctx, dir, "fork", "v1.0.0"))
	run(t, dir, "tag", "--delete", "v1.0.0")
	run(t, dir, "tag", "v1.0.0", head) // a different commit under the same name here

	err := PushTag(ctx, dir, "fork", "v1.0.0")

	require.ErrorIs(t, err, ErrPushRejected)
	assert.Equal(t, other, run(t, bare, "rev-parse", "refs/tags/v1.0.0"), "the remote's tag is where it was")
}

func TestPushTagOfATagThatIsNotThereIsAnError(t *testing.T) {
	dir, bare := forkOf(t)

	err := PushTag(ctx, dir, "fork", "v9.9.9")

	require.Error(t, err)
	assert.Empty(t, run(t, bare, "tag", "--list"))
}
