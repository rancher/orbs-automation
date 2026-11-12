package bundlestore

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTakesTheHighestReleaseNewerThanTheGeroWhateverWasFetchedLast(t *testing.T) {
	g := loadRig(t)
	g.cached("v0.7.0", t0.Add(-48*time.Hour), "seven")
	g.cached("v0.6.0", t0.Add(-time.Hour), "six")

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "v0.7.0", got.Source.Tag)
	assert.True(t, got.Source.FromCache)
}

func TestLoadPassesOverAnUnreadableReleaseForTheNextOneAndSaysWhy(t *testing.T) {
	g := loadRig(t)
	g.cached("v0.6.0", t0.Add(-48*time.Hour), "six")
	newer := strings.Replace(string(bundleOf(t, "v0.7.0", "seven")), "formatVersion: 1", "formatVersion: 2", 1)
	_, err := g.store.Put(Entry{Tag: "v0.7.0", Data: []byte(newer), FetchedAt: t0.Add(-time.Hour)})
	require.NoError(t, err)

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag, "the newest this gero can read")
	assert.Contains(t, got.Source.Note, "v0.7.0")
}

func TestLoadSkipsAPrereleaseThatTheChannelWouldNotChoose(t *testing.T) {
	g := loadRig(t)
	g.cached("v0.6.0", t0.Add(-48*time.Hour), "six")
	g.cached("v0.7.0-rc.1", t0.Add(-time.Hour), "seven")

	got, err := g.loadOffline("0.5.0", "")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag, "a stable gero is not given a release candidate")
}

func TestFetchKeepsTheNewestThreeBundles(t *testing.T) {
	g := newRig(t)
	for i, tag := range []string{"v0.6.0", "v0.7.0", "v0.8.0", "v0.9.0"} {
		g.publish(tag, "release-"+tag)
		g.now = t0.Add(time.Duration(i) * time.Hour)

		_, err := g.fetch("0.5.0")
		require.NoError(t, err)
	}

	var tags []string
	for _, e := range g.releaseEntries() {
		tags = append(tags, e.Tag)
	}
	assert.Equal(t, []string{"v0.9.0", "v0.8.0", "v0.7.0"}, tags, "the oldest is pruned")
}
