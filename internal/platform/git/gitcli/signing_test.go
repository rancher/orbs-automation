package gitcli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigBoolIsWhatGitWouldUseInTheClone(t *testing.T) {
	dir := clone(t, upstream(t))

	got, err := ConfigBool(ctx, dir, "commit.gpgSign")
	require.NoError(t, err)
	assert.False(t, got, "a key that is not set is false")

	run(t, dir, "config", "commit.gpgSign", "true")
	got, err = ConfigBool(ctx, dir, "commit.gpgSign")
	require.NoError(t, err)
	assert.True(t, got)

	run(t, dir, "config", "commit.gpgSign", "no")
	got, err = ConfigBool(ctx, dir, "commit.gpgSign")
	require.NoError(t, err)
	assert.False(t, got, "git's own spellings of a boolean")
}

func TestConfigBoolOfAValueThatIsNotABooleanIsAnError(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "commit.gpgSign", "sometimes")

	_, err := ConfigBool(ctx, dir, "commit.gpgSign")

	require.Error(t, err)
}

func TestSignsByDefault(t *testing.T) {
	dir := clone(t, upstream(t))

	commits, tags, err := SignsByDefault(ctx, dir)
	require.NoError(t, err)
	assert.False(t, commits)
	assert.False(t, tags)

	run(t, dir, "config", "commit.gpgSign", "true")
	commits, tags, err = SignsByDefault(ctx, dir)
	require.NoError(t, err)
	assert.True(t, commits)
	assert.False(t, tags, "commits and tags are set apart")

	run(t, dir, "config", "tag.gpgSign", "true")
	_, tags, err = SignsByDefault(ctx, dir)
	require.NoError(t, err)
	assert.True(t, tags)

	run(t, dir, "config", "tag.gpgSign", "false")
	run(t, dir, "config", "tag.forceSignAnnotated", "true")
	_, tags, err = SignsByDefault(ctx, dir)
	require.NoError(t, err)
	assert.True(t, tags, "git signs the annotated tags gero makes when this is set")
}
