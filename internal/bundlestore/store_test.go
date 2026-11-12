package bundlestore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
)

// bundleOf is the bundle of a release whose config is one project for each name.
func bundleOf(t *testing.T, tag string, names ...string) []byte {
	t.Helper()
	return bundletest.Compile(t, tag, bundletest.Config(names...))
}

func idOf(t *testing.T, data []byte) string {
	t.Helper()
	h, _, err := bundle.Parse(data)
	require.NoError(t, err)
	return h.ID()
}

func newStore(t *testing.T) Store { return Store{Dir: filepath.Join(t.TempDir(), "gero")} }

func TestStorePutKeepsTheBundleUnderTheIDOfItsHeader(t *testing.T) {
	s := newStore(t)
	data := bundleOf(t, "v0.6.0", "alpha")

	put, err := s.Put(Entry{Tag: "v0.6.0", Data: data, FetchedAt: t0})

	require.NoError(t, err)
	assert.Equal(t, idOf(t, data), put.ID)
	assert.NotEmpty(t, put.ID)
	got, ok := s.Get(put.ID)
	require.True(t, ok)
	assert.Equal(t, "v0.6.0", got.Tag)
	assert.Equal(t, string(data), string(got.Data))
	assert.True(t, got.FetchedAt.Equal(t0))
	h, _, _ := bundle.Parse(data)
	assert.Equal(t, h.InputsDigest, got.Digest)
	assert.Len(t, dirName(put.ID), 16, "the directory is named by the start of the ID")
}

func TestStoreHoldsSeveralBundlesAtOnceNewestFirst(t *testing.T) {
	s := newStore(t)
	for i, names := range [][]string{{"alpha"}, {"alpha", "bravo"}, {"charlie"}} {
		_, err := s.Put(Entry{Tag: "v0.6." + string(rune('0'+i)), Data: bundleOf(t, "v0.6."+string(rune('0'+i)), names...), FetchedAt: t0.Add(time.Duration(i) * time.Hour)})
		require.NoError(t, err)
	}

	got, err := s.List()

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"v0.6.2", "v0.6.1", "v0.6.0"}, []string{got[0].Tag, got[1].Tag, got[2].Tag})
}

func TestStorePutOfTheSameConfigReplacesTheEntry(t *testing.T) {
	s := newStore(t)
	first, err := s.Put(Entry{Tag: "v0.6.0", Data: bundleOf(t, "v0.6.0", "alpha"), FetchedAt: t0})
	require.NoError(t, err)

	second, err := s.Put(Entry{Tag: "v0.6.1", Data: bundleOf(t, "v0.6.1", "alpha"), FetchedAt: t0.Add(time.Hour)})

	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "the same files and compiler, whatever the version")
	got, err := s.List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "v0.6.1", got[0].Tag)
}

func TestStoreKeepsABundleThatHasNoDigestUnderANameFromItsBytes(t *testing.T) {
	s := newStore(t)
	old := string(bundleOf(t, "v0.5.0", "alpha"))
	for _, field := range []string{"inputsDigest", "compilerRevision"} {
		lines := strings.Split(old, "\n")
		kept := lines[:0]
		for _, l := range lines {
			if !strings.HasPrefix(l, field+":") {
				kept = append(kept, l)
			}
		}
		old = strings.Join(kept, "\n")
	}

	put, err := s.Put(Entry{Tag: "v0.5.0", Data: []byte(old), FetchedAt: t0})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(put.ID, "legacy-"), put.ID)
	_, ok := s.Get(put.ID)
	assert.True(t, ok)
}

func TestStoreKeepsABundleInAFormatThisGeroCannotRead(t *testing.T) {
	s := newStore(t)
	newer := strings.Replace(string(bundleOf(t, "v0.9.0", "alpha")), "formatVersion: 1", "formatVersion: 2", 1)

	put, err := s.Put(Entry{Tag: "v0.9.0", Data: []byte(newer), FetchedAt: t0})

	require.NoError(t, err)
	got, ok := s.Get(put.ID)
	require.True(t, ok, "a newer format is normal, not damage: the loader passes it over and says why")
	assert.Equal(t, newer, string(got.Data))
}

func TestStoreEntryThatWasDamagedIsTheSameAsNone(t *testing.T) {
	data := bundleOf(t, "v0.6.0", "alpha")
	files := func(s Store, id string) (bundlePath, metaPath string) {
		d := filepath.Join(s.Dir, bundlesDir, dirName(id))
		return filepath.Join(d, bundleFile), filepath.Join(d, metaFile)
	}
	tests := map[string]func(t *testing.T, bundlePath, metaPath string){
		"its bundle was edited": func(t *testing.T, b, _ string) {
			require.NoError(t, os.WriteFile(b, append(append([]byte{}, data...), "# edited\n"...), 0o644))
		},
		"its record is gone":    func(t *testing.T, _, m string) { require.NoError(t, os.Remove(m)) },
		"its bundle is gone":    func(t *testing.T, b, _ string) { require.NoError(t, os.Remove(b)) },
		"its record is garbage": func(t *testing.T, _, m string) { require.NoError(t, os.WriteFile(m, []byte("{nope"), 0o644)) },
		"its bundle is not a bundle, and its record agrees": func(t *testing.T, b, m string) {
			junk := []byte("not a bundle\n")
			require.NoError(t, os.WriteFile(b, junk, 0o644))
			raw, err := os.ReadFile(m)
			require.NoError(t, err)
			var rec map[string]any
			require.NoError(t, json.Unmarshal(raw, &rec))
			rec["sha256"] = hash(junk)
			out, err := json.Marshal(rec)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(m, out, 0o644))
		},
		"its bundle is another's, and its record agrees": func(t *testing.T, b, m string) {
			other := bundleOf(t, "v0.6.0", "bravo")
			require.NoError(t, os.WriteFile(b, other, 0o644))
			raw, err := os.ReadFile(m)
			require.NoError(t, err)
			var rec map[string]any
			require.NoError(t, json.Unmarshal(raw, &rec))
			rec["sha256"] = hash(other)
			out, err := json.Marshal(rec)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(m, out, 0o644))
		},
	}
	for name, damage := range tests {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			put, err := s.Put(Entry{Tag: "v0.6.0", Data: data, FetchedAt: t0})
			require.NoError(t, err)
			b, m := files(s, put.ID)
			require.FileExists(t, b, "the entry is where the test looks for it")

			damage(t, b, m)

			_, ok := s.Get(put.ID)
			assert.False(t, ok)
			list, err := s.List()
			require.NoError(t, err)
			assert.Empty(t, list)
		})
	}
}

func TestStoreTouchMovesTheTimeAndKeepsTheBundle(t *testing.T) {
	s := newStore(t)
	data := bundleOf(t, "v0.6.0", "alpha")
	put, err := s.Put(Entry{Tag: "v0.6.0", Data: data, FetchedAt: t0})
	require.NoError(t, err)

	require.NoError(t, s.Touch(put.ID, t0.Add(48*time.Hour)))

	got, ok := s.Get(put.ID)
	require.True(t, ok)
	assert.True(t, got.FetchedAt.Equal(t0.Add(48*time.Hour)))
	assert.Equal(t, string(data), string(got.Data))
}

func TestStorePruneKeepsTheNewestAndRemovesTheRestAndTheOldSingleEntry(t *testing.T) {
	s := newStore(t)
	var ids []string
	for i, name := range []string{"a", "b", "c", "d", "e"} {
		put, err := s.Put(Entry{Tag: "v0.6." + string(rune('0'+i)), Data: bundleOf(t, "v0.6."+string(rune('0'+i)), name), FetchedAt: t0.Add(time.Duration(i) * time.Hour)})
		require.NoError(t, err)
		ids = append(ids, put.ID)
	}
	require.NoError(t, os.WriteFile(filepath.Join(s.Dir, "bundle.yaml"), []byte("an older gero's entry\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(s.Dir, "meta.json"), []byte("{}"), 0o644))

	require.NoError(t, s.Prune(3))

	list, err := s.List()
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{ids[4], ids[3], ids[2]}, []string{list[0].ID, list[1].ID, list[2].ID})
	for _, gone := range ids[:2] {
		assert.NoDirExists(t, filepath.Join(s.Dir, bundlesDir, dirName(gone)))
	}
	assert.NoFileExists(t, filepath.Join(s.Dir, "bundle.yaml"))
	assert.NoFileExists(t, filepath.Join(s.Dir, "meta.json"))
}

func TestStorePutLeavesNoTemporaryFilesBehind(t *testing.T) {
	s := newStore(t)
	_, err := s.Put(Entry{Tag: "v0.6.0", Data: bundleOf(t, "v0.6.0", "alpha"), FetchedAt: t0})
	require.NoError(t, err)

	var names []string
	require.NoError(t, filepath.WalkDir(s.Dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, d.Name())
		}
		return err
	}))

	assert.ElementsMatch(t, []string{bundleFile, metaFile}, names)
}

func TestStoreWithNothingInItIsNotAnError(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "never-made")}

	list, err := s.List()

	require.NoError(t, err)
	assert.Empty(t, list)
	_, ok := s.Get("anything")
	assert.False(t, ok)
	require.NoError(t, s.Prune(3))
}

func TestStoreWithNoDirectoryKeepsNothingAndSaysNothing(t *testing.T) {
	s := Store{}

	put, err := s.Put(Entry{Tag: "v0.6.0", Data: bundleOf(t, "v0.6.0", "alpha"), FetchedAt: t0})

	require.NoError(t, err)
	_, ok := s.Get(put.ID)
	assert.False(t, ok)
	list, err := s.List()
	require.NoError(t, err)
	assert.Empty(t, list)
	require.NoError(t, s.Touch("x", t0))
	require.NoError(t, s.Prune(1))
}
