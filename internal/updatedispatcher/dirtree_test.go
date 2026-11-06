package updatedispatcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirTreeReadsAndWritesBelowTheDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.yaml"), []byte("image: old\n"), 0o640))
	tree := DirTree(dir)

	got, err := tree.ReadFile(context.Background(), "build.yaml")
	require.NoError(t, err)
	assert.Equal(t, "image: old\n", string(got))

	require.NoError(t, tree.WriteFile(context.Background(), "build.yaml", []byte("image: new\n")))
	data, err := os.ReadFile(filepath.Join(dir, "build.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "image: new\n", string(data))
	info, err := os.Stat(filepath.Join(dir, "build.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(), "the mode of an existing file is kept")
}

func TestDirTreeWriteCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, DirTree(dir).WriteFile(context.Background(), "a/b/c.txt", []byte("x")))

	data, err := os.ReadFile(filepath.Join(dir, "a", "b", "c.txt"))
	require.NoError(t, err)
	assert.Equal(t, "x", string(data))
}

func TestDirTreeReadOfAMissingFileIsAnError(t *testing.T) {
	_, err := DirTree(t.TempDir()).ReadFile(context.Background(), "nope.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope.yaml")
}

func TestDirTreeRefusesAPathOutsideTheDirectory(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o600))
	dir := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))
	tree := DirTree(dir)

	for _, path := range []string{"../secret", "a/../../secret", "/etc/passwd", "link/secret"} {
		_, err := tree.ReadFile(context.Background(), path)
		require.ErrorIs(t, err, ErrOutsideTree, "read %s", path)
		err = tree.WriteFile(context.Background(), path, []byte("x"))
		require.ErrorIs(t, err, ErrOutsideTree, "write %s", path)
	}
	data, err := os.ReadFile(filepath.Join(outside, "secret"))
	require.NoError(t, err)
	assert.Equal(t, "s", string(data), "nothing was written through the link")
}
