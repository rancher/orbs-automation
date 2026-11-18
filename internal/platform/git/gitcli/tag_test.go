package gitcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTagMakesAnAnnotatedTagAtTheCommitAndMovesNothing(t *testing.T) {
	dir := clone(t, upstream(t))
	main := run(t, dir, "rev-parse", "HEAD")
	release := run(t, dir, "rev-parse", "upstream/release/v1")

	require.NoError(t, CreateTag(ctx, dir, "v1.0.0-rc.1", release, Tag{Message: "Tag v1.0.0-rc.1\n\nAutomation: gero repo tag\n"}))

	assert.Equal(t, release, run(t, dir, "rev-parse", "refs/tags/v1.0.0-rc.1^{commit}"))
	assert.Equal(t, "tag", run(t, dir, "cat-file", "-t", "refs/tags/v1.0.0-rc.1"), "a tag object, not a bare ref")
	assert.Equal(t, "Tag v1.0.0-rc.1\n\nAutomation: gero repo tag", run(t, dir, "tag", "--list", "--format=%(contents)", "v1.0.0-rc.1"))
	assert.Equal(t, main, run(t, dir, "rev-parse", "HEAD"), "the checkout is where it was")
	assert.Equal(t, "main", run(t, dir, "branch", "--show-current"))
}

func TestCreateTagWithATagger(t *testing.T) {
	dir := clone(t, upstream(t))

	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "HEAD"), Tag{Message: "m", Name: "gero", Email: "gero@example.com"}))

	assert.Equal(t, "gero <gero@example.com>", run(t, dir, "tag", "--list", "--format=%(taggername) <%(taggeremail:trim)>", "v1.0.0"))
}

func TestCreateTagNeverMovesATagThatExists(t *testing.T) {
	dir := clone(t, upstream(t))
	first := run(t, dir, "rev-parse", "HEAD")
	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", first, Tag{Message: "m"}))

	err := CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "upstream/release/v1"), Tag{Message: "m"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, first, run(t, dir, "rev-parse", "refs/tags/v1.0.0^{commit}"))
}

func TestCreateTagAtACommitTheCloneLacksIsAnError(t *testing.T) {
	dir := clone(t, upstream(t))

	err := CreateTag(ctx, dir, "v1.0.0", "0123456789abcdef0123456789abcdef01234567", Tag{Message: "m"})

	require.Error(t, err)
	assert.Empty(t, run(t, dir, "tag", "--list"))
}

func TestCreateTagIsUnsignedWhenToldNotToWhateverGitIsConfiguredTo(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "tag.gpgSign", "true")
	run(t, dir, "config", "tag.forceSignAnnotated", "true")
	run(t, dir, "config", "gpg.program", "false") // signing would fail

	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "HEAD"), Tag{Message: "m", Sign: new(false)}))

	assert.NotContains(t, run(t, dir, "cat-file", "-p", "refs/tags/v1.0.0"), "SIGNATURE")
}

func TestCreateTagFollowsGitWhenNotToldWhetherToSign(t *testing.T) {
	dir := clone(t, upstream(t))
	head := run(t, dir, "rev-parse", "HEAD")
	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", head, Tag{Message: "m"}), "git is set to sign nothing")
	assert.NotContains(t, run(t, dir, "cat-file", "-p", "refs/tags/v1.0.0"), "SIGNATURE")

	run(t, dir, "config", "tag.gpgSign", "true")
	run(t, dir, "config", "gpg.program", "false") // signing would fail
	err := CreateTag(ctx, dir, "v1.0.1", head, Tag{Message: "m"})
	require.Error(t, err, "git is set to sign, and signing fails, so the tag is not made")
	assert.NotContains(t, run(t, dir, "tag", "--list"), "v1.0.1")
}

func TestCreateTagFollowsTagForceSignAnnotated(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "tag.forceSignAnnotated", "true")
	run(t, dir, "config", "gpg.program", "false") // signing would fail

	err := CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "HEAD"), Tag{Message: "m"})

	require.Error(t, err, "every annotated tag is signed, and signing fails")
}

func TestCreateTagSignsWhenToldTo(t *testing.T) {
	dir := clone(t, upstream(t))
	sshSigning(t, dir)

	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "HEAD"), Tag{Message: "m", Name: "gero", Email: "gero@example.com", Sign: new(true)}))

	assert.Contains(t, run(t, dir, "cat-file", "-p", "refs/tags/v1.0.0"), "-----BEGIN SSH SIGNATURE-----")
	run(t, dir, "verify-tag", "v1.0.0")
}

func TestCreateTagAsksedToSignFailsWithoutAKeyAndNeverFallsBackToUnsigned(t *testing.T) {
	dir := clone(t, upstream(t))
	run(t, dir, "config", "gpg.program", "false")

	err := CreateTag(ctx, dir, "v1.0.0", run(t, dir, "rev-parse", "HEAD"), Tag{Message: "m", Sign: new(true)})

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

// sshSigning sets the clone to sign with a new ssh key and to trust it, which
// needs no agent, no gpg and nothing outside the clone. It skips the test where
// git or ssh-keygen is too old to sign that way.
func sshSigning(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	key := filepath.Join(t.TempDir(), "key")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "gero@example.com", "-f", key).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen cannot make a key: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	require.NoError(t, err)
	allowed := filepath.Join(filepath.Dir(key), "allowed")
	require.NoError(t, os.WriteFile(allowed, []byte("gero@example.com "+string(pub)), 0o600))
	run(t, dir, "config", "gpg.format", "ssh")
	run(t, dir, "config", "user.signingkey", key+".pub")
	run(t, dir, "config", "gpg.ssh.allowedSignersFile", allowed)
}

func TestPushTagOfAnAnnotatedTagThatAnotherCloneMadeOnTheSameCommitIsRejected(t *testing.T) {
	dir, bare := forkOf(t)
	head := run(t, dir, "rev-parse", "HEAD")
	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", head, Tag{Message: "m"}))
	require.NoError(t, PushTag(ctx, dir, "fork", "v1.0.0"))
	run(t, dir, "tag", "--delete", "v1.0.0")
	time.Sleep(1100 * time.Millisecond) // the tagger date is in whole seconds
	require.NoError(t, CreateTag(ctx, dir, "v1.0.0", head, Tag{Message: "m"}))

	err := PushTag(ctx, dir, "fork", "v1.0.0")

	require.ErrorIs(t, err, ErrPushRejected, "another tag object, so it would move the remote's tag")
	assert.Equal(t, head, run(t, bare, "rev-parse", "refs/tags/v1.0.0^{commit}"))
}
