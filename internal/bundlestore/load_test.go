package bundlestore

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
)

// loadRig is a workstation that has only the bundle compiled into the binary, one
// project named "embedded", and an empty cache. It has no host at all: a Load that
// reaches for the network panics on the nil Releases.
func loadRig(t *testing.T) *rig {
	t.Helper()
	g := newRig(t)
	g.r.Releases = nil
	g.r.Embedded = bundletest.Config("embedded")
	return g
}

func (g *rig) loadOffline(version, channel string) (Loaded, error) {
	g.t.Helper()
	return g.r.Load(bg, Request{GeroVersion: version, Channel: channel})
}

// wantEmbedded says the load is the embedded bundle, of the release the gero is.
func wantEmbedded(t *testing.T, got Loaded, tag string) {
	t.Helper()
	assert.Equal(t, Source{Embedded: true, Tag: tag}, stripNote(got.Source))
	require.NotNil(t, got.FS)
	assert.Equal(t, 1, projects(t, got))
	assert.Equal(t, "embedded", got.Header.GeroVersion)
}

// stripNote drops the note, which the tests that care about it read themselves.
func stripNote(s Source) Source {
	s.Note = ""
	return s
}

func TestLoadWithNothingCachedIsTheEmbeddedBundleAndMakesNoRequest(t *testing.T) {
	g := loadRig(t)

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	wantEmbedded(t, got, "v0.5.0")
	assert.Empty(t, got.Source.Note)
}

func TestLoadADevBuildIsTheEmbeddedBundleWhateverIsCached(t *testing.T) {
	g := loadRig(t)
	g.cached("v9.0.0", t0.Add(-time.Hour), "newer")

	got, err := g.loadOffline("dev", "")

	require.NoError(t, err)
	wantEmbedded(t, got, "")
}

func TestLoadACachedBundleOfANewerReleaseIsUsed(t *testing.T) {
	g := loadRig(t)
	g.cached("v0.6.0", t0.Add(-3*time.Hour), "newer", "newest")

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, Source{Tag: "v0.6.0", FetchedAt: t0.Add(-3 * time.Hour), Age: 3 * time.Hour, FromCache: true}, got.Source)
	require.NotNil(t, got.FS)
	assert.Equal(t, 2, projects(t, got))
}

func TestLoadACachedBundleOfTheSameOrAnOlderReleaseIsNotUsed(t *testing.T) {
	for _, cached := range []string{"v0.5.0", "v0.4.0"} {
		t.Run(cached, func(t *testing.T) {
			g := loadRig(t)
			g.cached(cached, t0.Add(-time.Hour), "other")

			got, err := g.loadOffline("0.5.0", "")

			require.NoError(t, err)
			wantEmbedded(t, got, "v0.5.0")
		})
	}
}

func TestLoadTheCacheIsUsedAsOldAsItIsBecauseItIsOnlyReplacedByAFetch(t *testing.T) {
	g := loadRig(t)
	g.cached("v0.6.0", t0.Add(-90*24*time.Hour), "newer")

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
	assert.Equal(t, 90*24*time.Hour, got.Source.Age)
}

func TestLoadTheCacheMustBeOnTheLineTheGeroIsOn(t *testing.T) {
	tests := []struct {
		name, gero, channel, cached string
		wantCache                   bool
	}{
		{"a stable gero ignores a newer pre-release", "0.5.0", "", "v0.6.0-rc.1", false},
		{"a stable gero takes a newer stable", "0.5.0", "", "v0.5.1", true},
		{"a pre-release gero takes a newer pre-release", "0.6.0-rc.1", "", "v0.6.0-rc.2", true},
		{"a pre-release gero takes a newer stable", "0.6.0-rc.1", "", "v0.6.0", true},
		{"a pre-release gero ignores an older stable", "0.6.0-rc.1", "", "v0.5.9", false},
		{"the stable channel makes a pre-release gero ignore a newer pre-release", "0.6.0-rc.1", "stable", "v0.6.0-rc.2", false},
		{"the stable channel still takes a newer stable", "0.6.0-rc.1", "stable", "v0.6.0", true},
		{"the pre-release channel makes a stable gero take a newer pre-release", "0.5.0", "prerelease", "v0.6.0-rc.1", true},
		{"the pre-release channel still ignores an older one", "0.5.0", "prerelease", "v0.4.0-rc.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := loadRig(t)
			g.cached(tt.cached, t0.Add(-time.Hour), "other")

			got, err := g.loadOffline(tt.gero, tt.channel)

			require.NoError(t, err)
			assert.Equal(t, tt.wantCache, got.Source.FromCache)
			if tt.wantCache {
				assert.Equal(t, tt.cached, got.Source.Tag)
			}
		})
	}
}

func TestLoadACacheThisGeroCannotReadIsPassedOverAndTheNoteSaysSo(t *testing.T) {
	g := loadRig(t)
	newer := strings.Replace(string(bundleOf(t, "v0.6.0", "alpha")), "formatVersion: 1", "formatVersion: 2", 1)
	_, err := g.store.Put(Entry{Tag: "v0.6.0", Data: []byte(newer), FetchedAt: t0})
	require.NoError(t, err)

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	wantEmbedded(t, got, "v0.5.0")
	assert.Contains(t, got.Source.Note, "v0.6.0")
	assert.Contains(t, got.Source.Note, "embedded")
}

func TestLoadWithABrokenEmbeddedConfigIsAnError(t *testing.T) {
	g := loadRig(t)
	g.r.Embedded = fstest.MapFS{"broken.yaml": {Data: []byte("a: [\n")}}

	_, err := g.loadOffline("0.5.0", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the embedded bundle")
	assert.Contains(t, err.Error(), "does not validate")
}

func TestLoadWithNoEmbeddedBundleGivenIsTheOneCompiledIntoTheBinary(t *testing.T) {
	r := &Resolver{}

	got, err := r.Load(bg, Request{GeroVersion: "dev"})

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	require.NotNil(t, got.FS)
	assert.Positive(t, projects(t, got))
}

func TestSourceDescribesTheEmbeddedBundle(t *testing.T) {
	assert.Equal(t, "embedded bundle v0.5.0", Source{Embedded: true, Tag: "v0.5.0"}.Describe())
	assert.Equal(t, "embedded bundle (a build that is not a release)", Source{Embedded: true}.Describe())
}
