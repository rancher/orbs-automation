package gitcli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneMakesTheCloneWithTheNamedRemote(t *testing.T) {
	up := upstream(t)
	dir := filepath.Join(t.TempDir(), "rancher")

	err := Clone(ctx, up, dir, "upstream")

	require.NoError(t, err)
	assert.Equal(t, up, run(t, dir, "remote", "get-url", "upstream"))
	assert.Equal(t, "main", run(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	assert.FileExists(t, filepath.Join(dir, "build.yaml"))
}

func TestCloneOfAMissingRepositoryFailsWithWhatGitSaid(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "nope")

	err := Clone(ctx, filepath.Join(t.TempDir(), "absent"), dir, "upstream")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone")
	assert.NoDirExists(t, dir)
}

func TestAddRemoteAddsAndFetchesIt(t *testing.T) {
	up := upstream(t)
	other := upstream(t)
	dir := clone(t, up)

	err := AddRemote(ctx, dir, "fork", other)

	require.NoError(t, err)
	assert.Equal(t, other, run(t, dir, "remote", "get-url", "fork"))
	assert.NotEmpty(t, run(t, dir, "rev-parse", "fork/release/v1"), "the remote's branches were fetched")
}

func TestAddRemoteThatIsThereAlreadyFails(t *testing.T) {
	up := upstream(t)
	dir := clone(t, up)

	err := AddRemote(ctx, dir, "upstream", up)

	require.Error(t, err)
}

func TestRemoteURLIsTheURLOrEmptyForNoSuchRemote(t *testing.T) {
	up := upstream(t)
	dir := clone(t, up)

	got, err := RemoteURL(ctx, dir, "upstream")
	require.NoError(t, err)
	assert.Equal(t, up, got)

	got, err = RemoteURL(ctx, dir, "fork")
	require.NoError(t, err)
	assert.Empty(t, got)
}
