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
	"github.com/rancher/orbs-automation/internal/gero/assetfetch"
	"github.com/rancher/orbs-automation/internal/gero/assetfetch/fakehub"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

var bg = context.Background()

// rig is a workstation with an empty cache, a clock, a fake host with nothing
// published on it, and an embedded bundle of one project named "embedded".
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
		Embedded: bundletest.Compile(t, "embedded", bundletest.Config("embedded")),
	}
	return g
}

func (g *rig) publish(tag string, names ...string) {
	g.t.Helper()
	bundletest.Publish(g.t, g.hub, tag, bundletest.Config(names...))
}

// publishPrerelease publishes the release as a pre-release.
func (g *rig) publishPrerelease(tag string, names ...string) {
	g.t.Helper()
	bundletest.Publish(g.t, g.hub, tag, bundletest.Config(names...), fakehub.Prerelease())
}

// publishUnreadable publishes a release whose bundle is of a format this gero does
// not know.
func (g *rig) publishUnreadable(tag string, opts ...fakehub.Option) {
	g.t.Helper()
	newer := strings.Replace(string(bundletest.Compile(g.t, tag, bundletest.Config("alpha"))), "formatVersion: 1", "formatVersion: 2", 1)
	bundletest.PublishRaw(g.hub, tag, []byte(newer), opts...)
}

func (g *rig) fetch(version string) (Loaded, error) {
	g.t.Helper()
	return g.r.Fetch(bg, Request{GeroVersion: version})
}

func (g *rig) fetchOn(version, channel string) (Loaded, error) {
	g.t.Helper()
	return g.r.Fetch(bg, Request{GeroVersion: version, Channel: channel})
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

func (g *rig) nothingCached() {
	g.t.Helper()
	_, ok, err := g.cache.Read()
	require.NoError(g.t, err)
	assert.False(g.t, ok, "nothing is cached")
}

// verifying makes the rig refuse a release whose bundle and checksums are not signed.
func (g *rig) verifying() {
	g.r.Releases = assetfetch.New("rancher", "orbs-automation", assetfetch.Options{HTTP: g.hub.Client(), BaseURL: g.hub.URL(), Verifier: fakehub.Verifier{}})
}

func projects(t *testing.T, l Loaded) int {
	t.Helper()
	require.NotNil(t, l.FS)
	g, rep := chainload.Load(l.FS, config.Context{Now: bundletest.CompiledAt})
	require.NoError(t, rep.Err())
	return len(g.Projects())
}

func TestFetchTakesTheNewestStableBundleNewerThanTheGeroVerifiesItAndCachesIt(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "old")
	g.publish("v0.6.0", "alpha", "bravo")

	got, err := g.fetch("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Header.GeroVersion)
	assert.Equal(t, 2, projects(t, got))
	assert.Equal(t, Source{Tag: "v0.6.0", FetchedAt: t0}, got.Source)
	assert.Equal(t, "v0.6.0", g.cachedTag())
}

func TestFetchNeverPicksAPrereleaseBecauseItIsNewest(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publish("v0.5.1", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")

	got, err := g.fetch("v0.5.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.1", got.Source.Tag)
}

func TestFetchWithNothingNewerIsTheEmbeddedBundleAndDownloadsNothing(t *testing.T) {
	g := newRig(t)
	g.publish("v0.4.0", "old")
	g.publish("v0.5.0", "alpha")

	got, err := g.fetch("v0.5.0")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests("/releases/assets/"), "the bundle of the gero's own release is the embedded one")
	assert.Equal(t, Source{Embedded: true, Tag: "v0.5.0", Note: got.Source.Note}, got.Source)
	assert.Contains(t, got.Source.Note, "v0.5.0")
	assert.Equal(t, 1, projects(t, got))
	g.nothingCached()
}

func TestFetchADevBuildHasNoReleaseToBeNewerThanAndSaysSo(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")

	got, err := g.fetch("dev")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests(""), "no request is made")
	assert.True(t, got.Source.Embedded)
	assert.Contains(t, got.Source.Note, "not a release")
}

func TestFetchAPrereleaseGeroWhoseOwnBundleIsTheNewestUsesTheEmbeddedOne(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")

	got, err := g.fetch("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	assert.Equal(t, "v0.6.0-rc.1", got.Source.Tag)
	assert.Zero(t, g.hub.Requests("/releases/assets/"))
}

func TestFetchAPrereleaseGeroPrefersANewerStableBundle(t *testing.T) {
	g := newRig(t)
	g.publishPrerelease("v0.6.0-rc.1", "beta")
	g.publish("v0.6.0", "alpha")

	got, err := g.fetch("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
}

func TestFetchAPrereleaseGeroTakesTheNewestPrereleaseBundle(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")
	g.publishPrerelease("v0.6.0-rc.2", "beta", "gamma")

	got, err := g.fetch("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.2", got.Source.Tag)
	assert.Equal(t, 2, projects(t, got))
}

func TestFetchAPrereleaseGeroWorksWhenNoStableReleaseExists(t *testing.T) {
	g := newRig(t)
	g.publishPrerelease("v0.0.1-rc.1", "beta")
	g.publishPrerelease("v0.0.1-rc.2", "beta")

	got, err := g.fetch("0.0.1-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.0.1-rc.2", got.Source.Tag)
}

func TestFetchAStableGeroNeverTakesAPrereleaseWhenOnlyPrereleasesExist(t *testing.T) {
	g := newRig(t)
	g.publishPrerelease("v0.6.0-rc.1", "beta")

	_, err := g.fetch("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrNotFound)
}

func TestFetchWithNoReleaseAtAllIsUnavailable(t *testing.T) {
	g := newRig(t)

	_, err := g.fetch("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrNotFound)
}

// GoReleaser stamps {{.Version}} into the binary, which has no leading v, and the
// tags of the releases have one.
func TestFetchAGeroVersionWithoutTheLeadingVIsTheReleaseWithThatTag(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")

	got, err := g.fetch("0.6.0-rc.1")
	require.NoError(t, err)
	assert.True(t, got.Source.Embedded, "its own bundle is the newest")

	got, err = g.fetch("0.4.0")
	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
}

func TestFetchAlwaysAsksTheNetworkAndReplacesAFreshCache(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-time.Hour), "alpha")
	g.publish("v0.6.0", "alpha", "bravo")

	got, err := g.fetch("v0.4.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
	assert.False(t, got.Source.FromCache)
	assert.Equal(t, "v0.6.0", g.cachedTag())
}

func TestFetchTheCachedBundleOfTheNewestReleaseIsConfirmedWithoutDownloadingAgain(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.cached("v0.5.0", t0.Add(-30*time.Hour), "alpha")

	got, err := g.fetch("v0.4.0")

	require.NoError(t, err)
	assert.Zero(t, g.hub.Requests("/releases/assets/"), "the same release is not downloaded again")
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.Zero(t, got.Source.Age)
	e, _, err := g.cache.Read()
	require.NoError(t, err)
	assert.True(t, e.FetchedAt.Equal(t0), "the cache says it was checked now")
}

func TestFetchWhenTheNetworkFailsIsAnErrorAndTheCacheStaysAsItWas(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	g.hub.Fail("", 503)

	_, err := g.fetch("v0.4.0")

	require.ErrorIs(t, err, ErrUnavailable)
	assert.Equal(t, "v0.5.0", g.cachedTag())
}

func TestFetchWhenRateLimitedIsAnErrorThatSaysSo(t *testing.T) {
	g := newRig(t)
	g.publish("v0.6.0", "alpha")
	g.hub.Fail("/releases/latest", 403, "X-RateLimit-Remaining", "0")

	_, err := g.fetch("v0.5.0")

	require.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestFetchATamperedDownloadIsRefusedAndTheCacheStaysAsItWas(t *testing.T) {
	g := newRig(t)
	g.cached("v0.5.0", t0.Add(-72*time.Hour), "alpha")
	g.publish("v0.6.0", "alpha", "bravo")
	g.hub.Tamper("v0.6.0", bundle.Name, []byte("not the bundle\n"))

	_, err := g.fetch("v0.4.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrChecksumMismatch)
	assert.Equal(t, "v0.5.0", g.cachedTag(), "a mismatch never replaces the cache")
}

func TestFetchATamperedDownloadWithNoCacheCachesNothing(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.hub.Tamper("v0.5.0", bundle.Name, []byte("not the bundle\n"))

	_, err := g.fetch("v0.4.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrChecksumMismatch)
	g.nothingCached()
}

func TestFetchAReleaseThatPublishesNoChecksumForTheBundleIsNotUsed(t *testing.T) {
	g := newRig(t)
	data := bundletest.Compile(t, "v0.5.0", bundletest.Config("alpha"))
	bundletest.PublishRaw(g.hub, "v0.5.0", data, fakehub.ChecksumOf(bundle.Name, ""))

	_, err := g.fetch("v0.4.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrNoChecksum)
	g.nothingCached()
}

func TestFetchANewestBundleOfAFormatThisGeroCannotReadIsPassedOverForTheEmbeddedOne(t *testing.T) {
	g := newRig(t)
	g.publishUnreadable("v0.7.0")

	got, err := g.fetch("v0.5.0")

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.Contains(t, got.Source.Note, "v0.7.0")
	assert.Contains(t, got.Source.Note, "cannot read")
	assert.Contains(t, got.Source.Note, "format version 2", "the note names both versions")
	g.nothingCached()
}

func TestFetchAPrereleaseGeroPassesOverAnUnreadableNewerBundleForTheEmbeddedOne(t *testing.T) {
	g := newRig(t)
	g.publishPrerelease("v0.6.0-rc.1", "beta")
	g.publishUnreadable("v0.7.0")

	got, err := g.fetch("v0.6.0-rc.1")

	require.NoError(t, err)
	assert.True(t, got.Source.Embedded)
	assert.Equal(t, "v0.6.0-rc.1", got.Source.Tag)
	assert.Contains(t, got.Source.Note, "v0.7.0")
}

func TestFetchVerifiesTheSignaturesOfTheBundleAndTheChecksums(t *testing.T) {
	g := newRig(t)
	g.verifying()
	bundletest.Publish(t, g.hub, "v0.5.0", bundletest.Config("alpha"), fakehub.Signed("good"))

	got, err := g.fetch("v0.4.0")

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Source.Tag)
	assert.Equal(t, "v0.5.0", g.cachedTag())
}

func TestFetchAReleaseWithNoSignaturesIsRefusedAndNothingIsCached(t *testing.T) {
	g := newRig(t)
	g.verifying()
	g.publish("v0.5.0", "alpha")

	_, err := g.fetch("v0.4.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrSignatureMissing)
	g.nothingCached()
}

func TestFetchABadSignatureIsRefusedAndTheCacheStaysAsItWas(t *testing.T) {
	g := newRig(t)
	g.verifying()
	g.cached("v0.4.0", t0.Add(-48*time.Hour), "old")
	bundletest.Publish(t, g.hub, "v0.5.0", bundletest.Config("alpha"), fakehub.Signed("good"), fakehub.Signature("checksums.txt", "forged"))

	_, err := g.fetch("v0.3.0")

	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, assetfetch.ErrSignatureInvalid)
	assert.Equal(t, "v0.4.0", g.cachedTag())
}

func TestChannelStableMakesAPrereleaseGeroTakeANewerStableBundleOnly(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.3", "beta")

	got, err := g.fetchOn("v0.6.0-rc.2", ChannelStable)
	require.NoError(t, err)
	assert.True(t, got.Source.Embedded, "the stable v0.5.0 is older than the gero itself, and a release candidate is not asked for")

	g.publish("v0.6.0", "alpha", "bravo")
	got, err = g.fetchOn("v0.6.0-rc.2", ChannelStable)
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Source.Tag)
}

func TestChannelPrereleaseMakesAStableGeroTakeTheNewestPrereleaseBundle(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")

	got, err := g.fetchOn("v0.5.0", ChannelPrerelease)

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Source.Tag)
}

func TestChannelPrereleaseFallsBackToTheNewestNewerStableWhenTheNewestIsUnreadable(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publish("v0.5.1", "alpha")
	g.publishUnreadable("v0.7.0-rc.1", fakehub.Prerelease())

	got, err := g.fetchOn("v0.5.0", ChannelPrerelease)

	require.NoError(t, err)
	assert.Equal(t, "v0.5.1", got.Source.Tag)
	assert.Contains(t, got.Source.Note, "v0.7.0-rc.1")
}

func TestChannelAutoIsWhatTheGeroIs(t *testing.T) {
	g := newRig(t)
	g.publish("v0.5.0", "alpha")
	g.publishPrerelease("v0.6.0-rc.1", "beta")
	g.publishPrerelease("v0.6.0-rc.2", "beta")

	stable, err := g.fetchOn("v0.5.0", ChannelAuto)
	require.NoError(t, err)
	assert.True(t, stable.Source.Embedded, "a stable gero is not offered a release candidate")

	pre, err := g.fetchOn("v0.6.0-rc.1", "")
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.2", pre.Source.Tag)
}

func TestSourceDescribesTheReleaseAndHowOldTheCopyIs(t *testing.T) {
	assert.Equal(t, "bundle v0.5.0, 3 hours old", Source{Tag: "v0.5.0", Age: 3 * time.Hour}.Describe())
	assert.Equal(t, "bundle v0.5.0, 3 days old", Source{Tag: "v0.5.0", Age: 72 * time.Hour}.Describe())
	assert.Equal(t, "bundle v0.6.0, under a minute old", Source{Tag: "v0.6.0"}.Describe())
}

func TestASourceFromAFileDescribesTheFileAndTheRelease(t *testing.T) {
	s := Source{File: "/tmp/gero-config-bundle.yaml", Tag: "v0.3.0"}

	assert.Equal(t, "bundle file /tmp/gero-config-bundle.yaml (v0.3.0)", s.Describe())
}
