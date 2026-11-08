package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git/repo"
	"github.com/rancher/orbs-automation/internal/platform/git/repo/repotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// subdir creates and returns a directory below dir.
func subdir(t *testing.T, dir string) string {
	t.Helper()
	sub := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	return sub
}

// tagsOf lists the tags of r, which tells the test it got the right repository.
func tagsOf(t *testing.T, r *repo.Repo) []string {
	t.Helper()
	require.NotNil(t, r)
	tags, err := r.NewestFirst()
	require.NoError(t, err)
	return tags
}

func TestErrNotRepositoryIsTheRepoSentinel(t *testing.T) {
	assert.ErrorIs(t, ErrNotRepository, repo.ErrNotRepository)
}

func TestCurrent(t *testing.T) {
	dir, _ := repotest.NewMainRepo(t)

	t.Run("at the repository root", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), dir).Current()

		require.NoError(t, err)
		assert.Equal(t, []string{"main-1111111"}, tagsOf(t, r))
	})

	t.Run("from a subdirectory", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), subdir(t, dir)).Current()

		require.NoError(t, err)
		assert.Equal(t, []string{"main-1111111"}, tagsOf(t, r))
	})

	t.Run("outside any repository", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), t.TempDir()).Current()

		require.ErrorIs(t, err, ErrNotRepository)
		assert.Nil(t, r)
	})
}

func TestCurrentAtRoot(t *testing.T) {
	dir, _ := repotest.NewMainRepo(t)

	t.Run("at the repository root", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), dir).CurrentAtRoot()

		require.NoError(t, err)
		assert.Equal(t, []string{"main-1111111"}, tagsOf(t, r))
	})

	t.Run("a subdirectory is not the root", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), subdir(t, dir)).CurrentAtRoot()

		require.ErrorIs(t, err, ErrNotRepository)
		assert.Nil(t, r)
	})

	t.Run("outside any repository", func(t *testing.T) {
		r, err := Open(settings(t, "", nil, nil), t.TempDir()).CurrentAtRoot()

		require.ErrorIs(t, err, ErrNotRepository)
		assert.Nil(t, r)
	})
}

func TestOpenAt(t *testing.T) {
	dir := t.TempDir()
	repotest.NewRepoAt(t, dir, map[string]string{"origin": upstreamURL})

	r, err := Open(settings(t, "", nil, nil), t.TempDir()).OpenAt(dir)

	require.NoError(t, err)
	require.NotNil(t, r)
	remotes, err := r.Remotes()
	require.NoError(t, err)
	assert.Equal(t, []string{upstreamURL}, remotes["origin"], "it is the repository in dir, not the working directory's")
}

func TestOpenAtNotARepository(t *testing.T) {
	_, err := Open(settings(t, "", nil, nil), t.TempDir()).OpenAt(t.TempDir())
	assert.ErrorIs(t, err, ErrNotRepository)
}

func TestErrDetachedHeadIsTheRepositorysError(t *testing.T) {
	assert.ErrorIs(t, ErrDetachedHead, repo.ErrDetachedHead)
}
