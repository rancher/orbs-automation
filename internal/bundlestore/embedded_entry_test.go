package bundlestore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
)

// embeddedID is the ID the bundle of the config of the names has in the store.
func embeddedID(t *testing.T, names ...string) string {
	t.Helper()
	digest, err := bundle.Digest(bundletest.Config(names...))
	require.NoError(t, err)
	return bundle.Header{InputsDigest: digest, CompilerRevision: bundle.CompilerRevision}.ID()
}

func embeddedEntries(t *testing.T, g *rig) []Entry {
	t.Helper()
	list, err := g.store.List()
	require.NoError(t, err)
	var out []Entry
	for _, e := range list {
		if e.Origin == OriginEmbedded {
			out = append(out, e)
		}
	}
	return out
}

func TestLoadKeepsTheEmbeddedBundleInTheStoreUnderItsID(t *testing.T) {
	g := loadRig(t)

	_, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	entries := embeddedEntries(t, g)
	require.Len(t, entries, 1)
	assert.Equal(t, embeddedID(t, "embedded"), entries[0].ID)
	assert.Equal(t, "v0.5.0", entries[0].Tag, "the release of the gero it was compiled for")
	assert.True(t, entries[0].FetchedAt.Equal(t0), "when it was compiled, by the clock the resolver has")
}

func TestLoadTakesTheEmbeddedBundleFromTheStoreWhenItIsThere(t *testing.T) {
	g := loadRig(t)
	first, err := g.loadOffline("0.5.0", "")
	require.NoError(t, err)
	require.Equal(t, "embedded", first.Header.GeroVersion)

	// Compiling again would now make a bundle with this header: the stored one is used.
	g.r.EmbeddedHeader = bundle.Header{GeroVersion: "compiled-again", SourceCommit: "x", CompiledAt: bundletest.CompiledAt}
	second, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "embedded", second.Header.GeroVersion, "the same files and compiler are the same bundle")
	assert.True(t, second.Source.Embedded)
	assert.Len(t, embeddedEntries(t, g), 1)
}

func TestLoadCompilesTheEmbeddedBundleAgainWhenItsStoredCopyWasDamaged(t *testing.T) {
	g := loadRig(t)
	_, err := g.loadOffline("0.5.0", "")
	require.NoError(t, err)
	id := embeddedID(t, "embedded")
	path := filepath.Join(g.store.Dir, bundlesDir, dirName(id), bundleFile)
	require.NoError(t, os.WriteFile(path, []byte("edited by hand\n"), 0o644))
	g.r.EmbeddedHeader = bundle.Header{GeroVersion: "compiled-again", SourceCommit: "x", CompiledAt: bundletest.CompiledAt}

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "compiled-again", got.Header.GeroVersion, "a copy that does not match its record is as good as none")
	entries := embeddedEntries(t, g)
	require.Len(t, entries, 1)
	assert.Contains(t, string(entries[0].Data), "compiled-again", "and it is stored again")
}

func TestLoadOfOtherEmbeddedConfigIsAnotherEntry(t *testing.T) {
	g := loadRig(t)
	_, err := g.loadOffline("0.5.0", "")
	require.NoError(t, err)

	g.r.Embedded = bundletest.Config("other")
	_, err = g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	var ids []string
	for _, e := range embeddedEntries(t, g) {
		ids = append(ids, e.ID)
	}
	assert.ElementsMatch(t, []string{embeddedID(t, "embedded"), embeddedID(t, "other")}, ids)
}

func TestLoadWorksWhenTheStoreCannotBeWritten(t *testing.T) {
	g := loadRig(t)
	file := filepath.Join(t.TempDir(), "a-file")
	require.NoError(t, os.WriteFile(file, []byte("not a directory\n"), 0o644))
	g.r.Store = Store{Dir: filepath.Join(file, "gero")}

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	assert.Equal(t, 1, projects(t, got), "the bundle compiled in memory is used")
}

func TestAnEmbeddedEntryIsNeverTakenForANewerRelease(t *testing.T) {
	g := loadRig(t)
	_, err := g.store.Put(Entry{Tag: "v0.6.0", Origin: OriginEmbedded, Data: bundleOf(t, "v0.6.0", "other"), FetchedAt: t0})
	require.NoError(t, err)

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	assert.False(t, got.Source.FromCache)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}
