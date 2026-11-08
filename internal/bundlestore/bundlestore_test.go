package bundlestore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch"
	"github.com/rancher/orbs-automation/internal/platform/assetfetch/fakehub"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

var bg = context.Background()

// rig is a workstation with an empty cache, a clock, and a fake host with nothing
// published on it.
type rig struct {
	t     *testing.T
	hub   *fakehub.Hub
	cache Cache
	now   time.Time
	r     *Resolver
}

func newRig(t *testing.T) *rig {
	t.Helper()
	g := &rig{t: t, hub: fakehub.New(t), cache: Cache{Dir: t.TempDir()}, now: t0}
	g.r = &Resolver{
		Releases: assetfetch.New("rancher", "orbs-automation", assetfetch.Options{HTTP: g.hub.Client(), BaseURL: g.hub.URL()}),
		Cache:    g.cache,
		Now:      func() time.Time { return g.now },
	}
	return g
}

func (g *rig) publish(tag string, names ...string) []byte {
	g.t.Helper()
	return bundletest.Publish(g.t, g.hub, tag, bundletest.Config(names...))
}

func (g *rig) load(version string) (Loaded, error) {
	g.t.Helper()
	return g.r.Load(bg, Request{GeroVersion: version})
}

func (g *rig) offline(version string) (Loaded, error) {
	g.t.Helper()
	return g.r.Load(bg, Request{GeroVersion: version, Offline: true})
}

// cached puts a bundle of the release in the cache, fetched at the time.
func (g *rig) cached(tag string, at time.Time, names ...string) {
	g.t.Helper()
	data := bundletest.Compile(g.t, tag, bundletest.Config(names...))
	require.NoError(g.t, g.cache.Write(Entry{Tag: tag, Data: data, FetchedAt: at}))
}

func (g *rig) cachedTag() string {
	g.t.Helper()
	e, ok, err := g.cache.Read()
	require.NoError(g.t, err)
	require.True(g.t, ok, "something is in the cache")
	return e.Tag
}

func projects(t *testing.T, l Loaded) int {
	t.Helper()
	g, rep := chainload.Load(l.FS, config.Context{Now: bundletest.CompiledAt})
	require.NoError(t, rep.Err())
	return len(g.Projects())
}

func TestLoadFetchesTheNewestStableBundleVerifiesItAndCachesIt(t *testing.T) {
	g := newRig(t)
	g.publish("v0.4.0", "old")
	g.publish("v0.5.0", "alpha", "bravo")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Header.GeroVersion)
	assert.Equal(t, 2, projects(t, got))
	assert.Equal(t, Source{Tag: "v0.5.0", FetchedAt: t0}, got.Source)
	assert.Equal(t, "v0.5.0", g.cachedTag())
}

func TestLoadAStableGeroUsesTheNewestStableBundleEvenWhenNewerThanItself(t *testing.T) {
	g := newRig(t)
	g.publish("v0.4.0", "old")
	g.publish("v0.5.0", "alpha")

	got, err := g.load("v0.4.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}

func TestLoadNeverPicksAPrereleaseBecauseItIsNewest(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	bundletest.Publish(t, g.hub, "v0.6.0-rc.1", bundletest.Config("beta"), fakehub.Prerelease())

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}

func TestLoadADevBuildUsesTheNewestStableBundle(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")

	got, err := g.load("dev")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}

func TestLoadAPrereleaseGeroUsesItsOwnBundleWhenNoNewerStableExists(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	bundletest.Publish(t, g.hub, "v0.6.0-rc.1", bundletest.Config("beta"), fakehub.Prerelease())

	got, err := g.load("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Source.Tag, "made together with this gero")
	assert.Empty(t, got.Source.Note)
}

func TestLoadAPrereleaseGeroPrefersANewerStableBundle(t *testing.T) {
	g := newRig(t)
	bundletest.Publish(t, g.hub, "v0.6.0-rc.1", bundletest.Config("beta"), fakehub.Prerelease())
	g.publish("v0.6.0", "alpha")

	got, err := g.load("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
}

func TestLoadAPrereleaseGeroFallsBackToItsOwnWhenTheNewerBundleIsUnreadable(t *testing.T) {
	g := newRig(t)
	bundletest.Publish(t, g.hub, "v0.6.0-rc.1", bundletest.Config("beta"), fakehub.Prerelease())
	newer := strings.Replace(string(bundletest.Compile(t, "v0.7.0", bundletest.Config("alpha"))), "formatVersion: 1", "formatVersion: 2", 1)
	bundletest.PublishRaw(g.hub, "v0.7.0", []byte(newer))

	got, err := g.load("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Source.Tag)
	assert.Contains(t, got.Source.Note, "v0.7.0")
	assert.Contains(t, got.Source.Note, "cannot read")
}

func TestLoadUsesAFreshCacheWithoutTheNetwork(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.cached("v0.5.0", t0.Add(-3*time.Hour), "alpha")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests(""), "no request is made")
	assert.Equal(t, Source{Tag: "v0.5.0", FetchedAt: t0.Add(-3 * time.Hour), Age: 3 * time.Hour, FromCache: true}, got.Source)
}

func TestLoadRefreshesACacheOlderThanADay(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-25*time.Hour), "alpha")
	g.publish("v0.5.0", "alpha")
	g.publish("v0.6.0", "alpha", "bravo")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
	assert.False(t, got.Source.FromCache)
	assert.Equal(t, "v0.6.0", g.cachedTag())
}

func TestLoadAnOldCacheOfTheCurrentReleaseIsConfirmedWithoutDownloadingAgain(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.cached("v0.5.0", t0.Add(-30*time.Hour), "alpha")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests("/releases/assets/"), "the same release is not downloaded again")
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.False(t, got.Source.Stale)
	assert.Zero(t, got.Source.Age)
	e, _, err := g.cache.Read()
	require.NoError(t, err)
	assert.True(t, e.FetchedAt.Equal(t0), "the cache says it was checked now")
}

func TestLoadACacheOlderThanTheGerosOwnReleaseIsRefreshedEvenWhenFresh(t *testing.T) {
	g := newRig(t)
	g.cached("v0.4.0", t0.Add(-time.Hour), "old")
	g.publish("v0.5.0", "alpha")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}

func TestLoadWhenTheNetworkFailsTheCacheIsUsedAndSaysWhy(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	g.hub.Fail("", 503)

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.True(t, got.Source.FromCache)
	assert.True(t, got.Source.Stale)
	assert.Equal(t, 72*time.Hour, got.Source.Age)
	assert.Contains(t, got.Source.Note, "cached bundle of v0.5.0")
	assert.Contains(t, got.Source.Note, "3 days")
}

func TestLoadWhenRateLimitedTheCacheIsUsedAndTheNoteSaysSo(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	g.hub.Fail("/releases/latest", 403, "X-RateLimit-Remaining", "0")

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.True(t, got.Source.Stale)
	assert.Contains(t, got.Source.Note, "rate limit")
}

func TestLoadWithNothingCachedAndNoNetworkIsUnavailableAndSaysWhy(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.hub.Fail("/releases/latest", 403, "X-RateLimit-Remaining", "0")

	_, err := g.load("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestLoadATamperedDownloadIsRefusedAndTheCacheStaysAsItWas(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	g.publish("v0.6.0", "alpha", "bravo")
	g.hub.Tamper("v0.6.0", bundle.Name, []byte("not the bundle\n"))

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.True(t, got.Source.Stale)
	assert.Contains(t, got.Source.Note, "checksum")
	assert.Equal(t, "v0.5.0", g.cachedTag(), "a mismatch never replaces the cache")
}

func TestLoadATamperedDownloadWithNoCacheIsUnavailable(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.hub.Tamper("v0.5.0", bundle.Name, []byte("not the bundle\n"))

	_, err := g.load("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrChecksumMismatch)
	_, ok, rerr := g.cache.Read()
	require.NoError(t, rerr)
	assert.False(t, ok, "nothing unverified is cached")
}

func TestLoadAReleaseThatPublishesNoChecksumForTheBundleIsNotUsed(t *testing.T) {
	g := newRig(t)
	data := bundletest.Compile(t, "v0.5.0", bundletest.Config("alpha"))
	bundletest.PublishRaw(g.hub, "v0.5.0", data, fakehub.ChecksumOf(bundle.Name, ""))

	_, err := g.load("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrNoChecksum)
}

func TestLoadABundleOfAFormatThisGeroCannotReadIsUnavailableAndNamesBothVersions(t *testing.T) {
	g := newRig(t)
	newer := strings.Replace(string(bundletest.Compile(t, "v0.7.0", bundletest.Config("alpha"))), "formatVersion: 1", "formatVersion: 2", 1)
	bundletest.PublishRaw(g.hub, "v0.7.0", []byte(newer))

	_, err := g.load("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, bundle.ErrUnsupported)
	assert.Contains(t, err.Error(), "format version 2")
}

func TestLoadAnUnreadableNewestBundleFallsBackToTheCacheAndSaysSo(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	newer := strings.Replace(string(bundletest.Compile(t, "v0.7.0", bundletest.Config("alpha"))), "formatVersion: 1", "formatVersion: 2", 1)
	bundletest.PublishRaw(g.hub, "v0.7.0", []byte(newer))

	got, err := g.load("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.Contains(t, got.Source.Note, "cannot read")
}

func TestLoadOfflineReadsTheCacheAndMakesNoRequest(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.cached("v0.5.0", t0.Add(-3*time.Hour), "alpha")

	got, err := g.offline("v0.5.0")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests(""))
	assert.True(t, got.Source.FromCache)
	assert.False(t, got.Source.Stale)
}

func TestLoadOfflineSaysStaleForACachePastItsRefreshTime(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")

	got, err := g.offline("v0.5.0")

	require.NoError(t, err)
	assert.True(t, got.Source.Stale)
	assert.Contains(t, got.Source.Note, "3 days")
}

func TestLoadOfflineWithNothingCachedIsUnavailableAndMakesNoRequest(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")

	_, err := g.offline("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	assert.Zero(t, g.hub.Requests(""))
}

func TestLoadWithNoReleaseAtAllIsUnavailable(t *testing.T) {
	g := newRig(t)

	_, err := g.load("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrNotFound)
}
