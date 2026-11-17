package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func tags(names ...string) []Tag {
	out := make([]Tag, len(names))
	for i, n := range names {
		out[i] = Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

func tagNamed(name string) *Tag { return &Tag{Name: name, Commit: "sha-" + name} }

func TestLatestReleases(t *testing.T) {
	got := LatestReleases(tags(
		"main-abc1234",    // not a version
		"v2.15.0-rc.1",    // older than the stable tag below: history
		"v2.14.8",         // a patch on an older line
		"v2.15.2",         // the highest stable
		"v2.9.1",          // 9 < 15 numerically
		"v2.16.0-alpha10", // the highest prerelease, and newer than the stable tag
		"v2.16.0-alpha9",
		"v2.15.2-rc.1",
		"release-notes",
	), "v*")

	assert.Equal(t, Latest{Stable: tagNamed("v2.15.2"), Prerelease: tagNamed("v2.16.0-alpha10")}, got)
}

func TestLatestReleasesKeepsTheCommit(t *testing.T) {
	got := LatestReleases([]Tag{{Name: "v1.0.0", Commit: "abc"}}, "v*")
	assert.Equal(t, &Tag{Name: "v1.0.0", Commit: "abc"}, got.Stable)
}

func TestLatestReleasesDropsASupersededPrerelease(t *testing.T) {
	got := LatestReleases(tags("v2.15.3", "v2.15.3-rc2"), "v*")
	assert.Equal(t, Latest{Stable: tagNamed("v2.15.3")}, got)
}

func TestLatestReleasesHonorsTheTagPattern(t *testing.T) {
	all := tags("v1.0.0", "foo-v3.0.0", "9.9.9", "v1.1.0-rc.1")

	assert.Equal(t, Latest{Stable: tagNamed("v1.0.0"), Prerelease: tagNamed("v1.1.0-rc.1")}, LatestReleases(all, "v*"))
	assert.Equal(t, Latest{Stable: tagNamed("9.9.9")}, LatestReleases(all, "[0-9]*"))
}

func TestLatestReleasesWithNothingToFind(t *testing.T) {
	assert.Equal(t, Latest{}, LatestReleases(nil, "v*"))
	assert.Equal(t, Latest{}, LatestReleases(tags("main-abc"), "v*"))
	assert.Equal(t, Latest{}, LatestReleases(tags("v1.0.0"), "["), "an invalid pattern matches nothing")
}

func TestLatestReleasesOnlyStable(t *testing.T) {
	assert.Equal(t, Latest{Stable: tagNamed("v1.2.0")}, LatestReleases(tags("v1.1.0", "v1.2.0"), "v*"))
}

func TestLatestReleasesOnlyPrerelease(t *testing.T) {
	assert.Equal(t, Latest{Prerelease: tagNamed("v0.1.0-rc.2")}, LatestReleases(tags("v0.1.0-rc.1", "v0.1.0-rc.2"), "v*"))
}

func TestLatestNewest(t *testing.T) {
	stable, pre := tagNamed("v1.0.0"), tagNamed("v1.1.0-rc.1")
	assert.Equal(t, pre, Latest{Stable: stable, Prerelease: pre}.Newest(), "a prerelease is reported only when it is newer")
	assert.Equal(t, stable, Latest{Stable: stable}.Newest())
	assert.Equal(t, pre, Latest{Prerelease: pre}.Newest())
	assert.Nil(t, Latest{}.Newest())
}

func TestLatestReleasesOfK3sTakesTheNewestPatchAndTheNewestRespin(t *testing.T) {
	all := tags(
		"v1.35.0+k3s1", "v1.35.0+k3s3", "v1.35.9+k3s1", "v1.35.10-rc1+k3s1", "v1.35.10+k3s1", "v1.35.9-rc2+k3s1",
		"v1.36.0+k3s1", "v1.34.9+k3s2", "v1.34.9+k3s1",
	)

	assert.Equal(t, tagNamed("v1.35.10+k3s1"), LatestReleases(all, "v1.35.*").Stable, "10 beats 9, and no release candidate wins")
	assert.Nil(t, LatestReleases(all, "v1.35.*").Prerelease, "an older candidate is history")
	assert.Equal(t, tagNamed("v1.34.9+k3s2"), LatestReleases(all, "v1.34.*").Stable, "a respin beats the first release of a patch")
	assert.Equal(t, tagNamed("v1.35.0+k3s3"), LatestReleases(tags("v1.35.0+k3s1", "v1.35.0+k3s3"), "v1.35.*").Stable, "whichever order the tags come in")
	assert.Equal(t, tagNamed("v1.35.0+k3s3"), LatestReleases(tags("v1.35.0+k3s3", "v1.35.0+k3s1"), "v1.35.*").Stable)
}
