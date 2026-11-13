package assetfetch

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/gero/assetfetch/fakehub"
)

var bg = context.Background()

func fetcher(h *fakehub.Hub) *Fetcher {
	return New("rancher", "orbs-automation", Options{HTTP: h.Client(), BaseURL: h.URL()})
}

func TestLatestIsTheNewestStableReleaseNotAPrereleaseOrADraft(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.4.0")
	h.Publish("v0.5.0")
	h.Publish("v0.6.0-rc.1", fakehub.Prerelease())
	h.Publish("v0.6.0", fakehub.Draft())

	got, err := fetcher(h).Latest(bg, false)

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.Tag)
	assert.False(t, got.Prerelease)
	assert.False(t, got.PublishedAt.IsZero())
}

func TestLatestWithNoStableReleaseIsNotFound(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.1.0-rc.1", fakehub.Prerelease())

	_, err := fetcher(h).Latest(bg, false)

	require.ErrorIs(t, err, ErrNotFound)
}

func TestLatestRateLimitedSaysSo(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0")
	h.Fail("/releases/latest", 403, "X-RateLimit-Remaining", "0")

	_, err := fetcher(h).Latest(bg, false)

	require.ErrorIs(t, err, ErrRateLimited)
}

func TestReleaseGivesTheTagAndWhetherItIsAPrerelease(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.6.0-rc.1", fakehub.Prerelease())

	got, err := fetcher(h).Release(bg, "v0.6.0-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag)
	assert.True(t, got.Prerelease)
}

func TestReleaseOfATagThatWasNeverPublishedIsNotFound(t *testing.T) {
	h := fakehub.New(t)

	_, err := fetcher(h).Release(bg, "v9.9.9")

	require.ErrorIs(t, err, ErrNotFound)
}

func TestAssetIsReturnedWhenItMatchesThePublishedChecksum(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")), fakehub.Asset("other", []byte("x")))

	got, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.NoError(t, err)
	assert.Equal(t, "the bundle\n", string(got))
}

func TestAssetThatWasTamperedWithIsAMismatchAndNothingIsReturned(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")))
	h.Tamper("v0.5.0", "gero-config-bundle.yaml", []byte("not the bundle\n"))

	got, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrChecksumMismatch)
	assert.Nil(t, got)
}

func TestAssetWithAWrongPublishedHashIsAMismatch(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0",
		fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")),
		fakehub.ChecksumOf("gero-config-bundle.yaml", strings.Repeat("0", 64)))

	got, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrChecksumMismatch)
	assert.Nil(t, got)
}

func TestAssetWithNoPublishedChecksumIsRefused(t *testing.T) {
	for name, opt := range map[string]fakehub.Option{
		"left out of checksums.txt": fakehub.ChecksumOf("gero-config-bundle.yaml", ""),
		"no checksums.txt at all":   fakehub.WithoutChecksums(),
	} {
		h := fakehub.New(t)
		h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")), opt)

		got, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

		require.ErrorIs(t, err, ErrNoChecksum, name)
		assert.Nil(t, got, name)
	}
}

func TestAssetThatIsNotOnTheReleaseIsNotFound(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("other", []byte("x")))

	_, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrAssetNotFound)
}

func TestAssetOfAReleaseThatIsNotThereIsNotFound(t *testing.T) {
	h := fakehub.New(t)

	_, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrNotFound)
}

func TestCompareVersionsOrdersTagsAndSaysWhenOneIsNotAVersion(t *testing.T) {
	cmp, ok := CompareVersions("v0.5.0", "v0.6.0")
	assert.True(t, ok)
	assert.Negative(t, cmp)

	cmp, ok = CompareVersions("v0.6.0-rc.1", "v0.6.0")
	assert.True(t, ok)
	assert.Negative(t, cmp, "a prerelease is older than its release")

	cmp, ok = CompareVersions("v1.0.0", "1.0.0")
	assert.True(t, ok)
	assert.Zero(t, cmp)

	_, ok = CompareVersions("dev", "v0.5.0")
	assert.False(t, ok)
}

func TestLatestWithPrereleasesIsTheNewestByVersionStableOrNot(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0")
	h.Publish("v0.6.0-rc.1", fakehub.Prerelease())
	h.Publish("v0.6.0-rc.2", fakehub.Prerelease())
	h.Publish("v0.7.0", fakehub.Draft())

	got, err := fetcher(h).Latest(bg, true)

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.2", got.Tag)
	assert.True(t, got.Prerelease)
}

func TestLatestWithPrereleasesPrefersTheStableReleaseOfTheSameVersion(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.6.0-rc.1", fakehub.Prerelease())
	h.Publish("v0.6.0")

	got, err := fetcher(h).Latest(bg, true)

	require.NoError(t, err)
	assert.Equal(t, "v0.6.0", got.Tag)
}

func TestLatestWithPrereleasesWorksWhenOnlyPrereleasesExist(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.0.1-rc.1", fakehub.Prerelease())
	h.Publish("v0.0.1-rc.2", fakehub.Prerelease())

	got, err := fetcher(h).Latest(bg, true)

	require.NoError(t, err)
	assert.Equal(t, "v0.0.1-rc.2", got.Tag)
}

func signedFetcher(h *fakehub.Hub) *Fetcher {
	return New("rancher", "orbs-automation", Options{HTTP: h.Client(), BaseURL: h.URL(), Verifier: fakehub.Verifier{}})
}

func TestAssetWithAVerifierIsReturnedWhenTheChecksumAndTheSignaturesHold(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")), fakehub.Signed("good"))

	got, err := signedFetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.NoError(t, err)
	assert.Equal(t, "the bundle\n", string(got))
}

func TestAssetWithAVerifierRefusesAReleaseThatIsNotSigned(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")))

	_, err := signedFetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrSignatureMissing)
}

func TestAssetWithAVerifierRefusesAMissingSignatureOfTheAssetAlone(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")),
		fakehub.Signed("good"), fakehub.Signature("gero-config-bundle.yaml", ""))

	_, err := signedFetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrSignatureMissing)
}

func TestAssetWithAVerifierRefusesABadSignatureOnTheAsset(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")),
		fakehub.Signed("good"), fakehub.Signature("gero-config-bundle.yaml", "forged"))

	_, err := signedFetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestAssetWithAVerifierRefusesABadSignatureOnTheChecksums(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")),
		fakehub.Signed("good"), fakehub.Signature("checksums.txt", "forged"))

	_, err := signedFetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestAssetWithNoVerifierDoesNotLookForSignatures(t *testing.T) {
	h := fakehub.New(t)
	h.Publish("v0.5.0", fakehub.Asset("gero-config-bundle.yaml", []byte("the bundle\n")))

	_, err := fetcher(h).Asset(bg, "v0.5.0", "gero-config-bundle.yaml")

	require.NoError(t, err)
	assert.Zero(t, h.Requests(".sigstore.json"))
}
