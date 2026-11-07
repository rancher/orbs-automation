package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tags(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

func lined() config.Project {
	return config.Project{
		Name: "kuberlr-kubectl",
		Repo: config.Repo{Name: "rancher/kuberlr-kubectl", Owned: true},
		Release: config.Release{TagPattern: "v*", Lines: []config.ProjectLine{
			{Line: "v2.16", Version: "9.x"}, {Line: "v2.15", Version: "8.x"},
		}},
	}
}

func universal() config.Project {
	return config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator", Owned: true}, Release: config.Release{TagPattern: "v*"}}
}

var sccTags = tags("v0.5.2", "v0.5.3-rc.1", "v0.5.3", "v0.6.0-rc.1", "main-abc1234")

func TestResolveReleaseLatestStableByDefault(t *testing.T) {
	got, err := ResolveRelease(universal(), sccTags, "", false, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, Release{Tag: "v0.5.3", Commit: "sha-v0.5.3", Stable: true}, got)
}

func TestResolveReleaseLatestWithPrerelease(t *testing.T) {
	got, err := ResolveRelease(universal(), sccTags, "", true, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "newer than the latest stable")
	assert.False(t, got.Stable)
}

func TestResolveReleaseAnExplicitTag(t *testing.T) {
	got, err := ResolveRelease(universal(), sccTags, "v0.5.2", false, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, Release{Tag: "v0.5.2", Commit: "sha-v0.5.2", Stable: true}, got, "an older tag is used as given")
}

func TestResolveReleaseATagThatDoesNotExist(t *testing.T) {
	_, err := ResolveRelease(universal(), sccTags, "v9.9.9", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "v9.9.9")
}

func TestResolveReleaseALine(t *testing.T) {
	all := tags("v8.0.1", "v8.1.0", "v8.2.0-rc.1", "v9.0.0", "v9.1.0-rc.1")

	stable, err := ResolveRelease(lined(), all, "v2.15", false, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v8.1.0", stable.Tag, "the latest stable tag of 8.x")
	assert.Equal(t, "v2.15", stable.Line)

	withPre, err := ResolveRelease(lined(), all, "v2.15", true, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v8.2.0-rc.1", withPre.Tag)

	current, err := ResolveRelease(lined(), all, "v2.16", false, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v9.0.0", current.Tag)
}

func TestResolveReleaseALineWithNoTags(t *testing.T) {
	_, err := ResolveRelease(lined(), tags("v9.0.0"), "v2.15", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "v2.15")
}

func TestResolveReleaseNeitherATagNorALine(t *testing.T) {
	_, err := ResolveRelease(lined(), tags("v9.0.0"), "latest", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "neither a tag nor a release line")

	_, err = ResolveRelease(universal(), sccTags, "v2.15", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrNoRelease, "a project with no lines has no line to name")
}

func TestResolveReleaseNothingReleasedYet(t *testing.T) {
	_, err := ResolveRelease(universal(), tags("main-abc1234"), "", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrNoRelease)
}

func TestResolveReleaseAPrereleaseTag(t *testing.T) {
	_, err := ResolveRelease(universal(), sccTags, "v0.6.0-rc.1", false, config.AcceptStable)
	assert.ErrorIs(t, err, ErrPrerelease, "the consumer takes stable releases only")
	assert.ErrorContains(t, err, "--prerelease")

	got, err := ResolveRelease(universal(), sccTags, "v0.6.0-rc.1", true, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "asked for")

	got, err = ResolveRelease(universal(), sccTags, "v0.6.0-rc.1", false, config.AcceptAll)
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "a consumer that accepts all releases")
}

func TestResolveReleaseFillsTheLineOfATag(t *testing.T) {
	got, err := ResolveRelease(lined(), tags("v8.1.0"), "v8.1.0", false, config.AcceptStable)
	require.NoError(t, err)
	assert.Equal(t, "v2.15", got.Line)
}
