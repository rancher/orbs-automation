package bundlestore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

func entry(tag, data string) Entry { return Entry{Tag: tag, Data: []byte(data), FetchedAt: t0} }

func TestCacheGivesBackWhatWasWritten(t *testing.T) {
	c := Cache{Dir: filepath.Join(t.TempDir(), "gero")}

	require.NoError(t, c.Write(entry("v0.5.0", "bundle one\n")))

	got, ok, err := c.Read()
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, entry("v0.5.0", "bundle one\n"), got)
}

func TestCacheWithNothingInItIsNotAnError(t *testing.T) {
	got, ok, err := Cache{Dir: filepath.Join(t.TempDir(), "never-made")}.Read()

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, Entry{}, got)
}

func TestCacheWriteReplacesTheOldEntry(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	require.NoError(t, c.Write(entry("v0.5.0", "old\n")))

	require.NoError(t, c.Write(entry("v0.6.0", "new\n")))

	got, ok, err := c.Read()
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "v0.6.0", got.Tag)
	assert.Equal(t, "new\n", string(got.Data))
}

func TestCacheWriteLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	c := Cache{Dir: dir}

	require.NoError(t, c.Write(entry("v0.5.0", "x\n")))
	require.NoError(t, c.Write(entry("v0.6.0", "y\n")))

	names, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, names, 2, "the bundle and what says which release it is")
}

func TestCacheThatWasDamagedIsTheSameAsNone(t *testing.T) {
	for name, damage := range map[string]func(dir string){
		"the bundle was edited": func(dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte("edited\n"), 0o644))
		},
		"the bundle is gone": func(dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "bundle.yaml")))
		},
		"the record is junk": func(dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte("{not json"), 0o644))
		},
		"the record is gone": func(dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "meta.json")))
		},
	} {
		dir := t.TempDir()
		c := Cache{Dir: dir}
		require.NoError(t, c.Write(entry("v0.5.0", "bundle\n")))
		damage(dir)

		got, ok, err := c.Read()

		require.NoError(t, err, name)
		assert.False(t, ok, name)
		assert.Equal(t, Entry{}, got, name)
	}
}

func TestCacheConfirmMovesTheTimeAndKeepsTheBundle(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	require.NoError(t, c.Write(entry("v0.5.0", "bundle\n")))

	require.NoError(t, c.Confirm(t0.Add(48*time.Hour)))

	got, ok, err := c.Read()
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "v0.5.0", got.Tag)
	assert.Equal(t, "bundle\n", string(got.Data))
	assert.True(t, got.FetchedAt.Equal(t0.Add(48*time.Hour)), got.FetchedAt.String())
}

func TestCacheWithNoDirectoryKeepsNothingAndSaysNothing(t *testing.T) {
	c := Cache{}

	require.NoError(t, c.Write(entry("v0.5.0", "x\n")))
	require.NoError(t, c.Confirm(t0))

	_, ok, err := c.Read()
	require.NoError(t, err)
	assert.False(t, ok)
}
