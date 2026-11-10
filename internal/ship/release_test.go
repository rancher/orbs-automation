package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/tagscheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveRelease is ResolveRelease for the tag scheme of the producer's stream.
func resolveRelease(t *testing.T, p projectSpec, all []git.Tag, arg string, prerelease bool, accept string) (Release, error) {
	t.Helper()
	ws := &fakeWorkspace{t: t, producer: p}
	project, err := ws.Project("")
	require.NoError(t, err)
	scheme, err := tagscheme.ForProject(project)
	require.NoError(t, err)
	return ResolveRelease(scheme, project.Slug(), all, arg, prerelease, accept == "all")
}

var sccTags = tags("v0.5.2", "v0.5.3-rc.1", "v0.5.3", "v0.6.0-rc.1", "main-abc1234")

func TestResolveReleaseLatestStableByDefault(t *testing.T) {
	got, err := resolveRelease(t, universal(), sccTags, "", false, "stable")
	require.NoError(t, err)
	assert.Equal(t, Release{Tag: "v0.5.3", Commit: "sha-v0.5.3", Stable: true}, got)
}

func TestResolveReleaseLatestWithPrerelease(t *testing.T) {
	got, err := resolveRelease(t, universal(), sccTags, "", true, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "newer than the latest stable")
	assert.False(t, got.Stable)
}

func TestResolveReleaseAnExplicitTag(t *testing.T) {
	got, err := resolveRelease(t, universal(), sccTags, "v0.5.2", false, "stable")
	require.NoError(t, err)
	assert.Equal(t, Release{Tag: "v0.5.2", Commit: "sha-v0.5.2", Stable: true}, got, "an older tag is used as given")
}

func TestResolveReleaseATagThatDoesNotExist(t *testing.T) {
	_, err := resolveRelease(t, universal(), sccTags, "v9.9.9", false, "stable")
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "v9.9.9")
}

func TestResolveReleaseALine(t *testing.T) {
	all := tags("v8.0.1", "v8.1.0", "v8.2.0-rc.1", "v9.0.0", "v9.1.0-rc.1")

	stable, err := resolveRelease(t, lined(), all, "v2.15", false, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v8.1.0", stable.Tag, "the latest stable tag of 8.x")
	assert.Equal(t, "v2.15", stable.Line)

	withPre, err := resolveRelease(t, lined(), all, "v2.15", true, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v8.2.0-rc.1", withPre.Tag)

	current, err := resolveRelease(t, lined(), all, "v2.16", false, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v9.0.0", current.Tag)
}

func TestResolveReleaseALineWithNoTags(t *testing.T) {
	_, err := resolveRelease(t, lined(), tags("v9.0.0"), "v2.15", false, "stable")
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "v2.15")
}

func TestResolveReleaseNeitherATagNorALine(t *testing.T) {
	_, err := resolveRelease(t, lined(), tags("v9.0.0"), "latest", false, "stable")
	assert.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "neither a tag nor a release line")

	_, err = resolveRelease(t, universal(), sccTags, "v2.15", false, "stable")
	assert.ErrorIs(t, err, ErrNoRelease, "a project with no lines has no line to name")
}

func TestResolveReleaseNothingReleasedYet(t *testing.T) {
	_, err := resolveRelease(t, universal(), tags("main-abc1234"), "", false, "stable")
	assert.ErrorIs(t, err, ErrNoRelease)
}

func TestResolveReleaseAPrereleaseTag(t *testing.T) {
	_, err := resolveRelease(t, universal(), sccTags, "v0.6.0-rc.1", false, "stable")
	assert.ErrorIs(t, err, ErrPrerelease, "the consumer takes stable releases only")
	assert.ErrorContains(t, err, "--prerelease")

	got, err := resolveRelease(t, universal(), sccTags, "v0.6.0-rc.1", true, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "asked for")

	got, err = resolveRelease(t, universal(), sccTags, "v0.6.0-rc.1", false, "all")
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Tag, "a consumer that accepts all releases")
}

func TestResolveReleaseFillsTheLineOfATag(t *testing.T) {
	got, err := resolveRelease(t, lined(), tags("v8.1.0"), "v8.1.0", false, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v2.15", got.Line)
}

func TestResolveReleaseAStreamWithOnlyPrereleasesNamesTheFlag(t *testing.T) {
	rcs := tags("v0.7.0-rc.1", "v0.7.0-rc.2")

	_, err := resolveRelease(t, universal(), rcs, "", false, "stable")

	require.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "no stable release")
	assert.ErrorContains(t, err, "v0.7.0-rc.2", "it names the newest there is")
	assert.ErrorContains(t, err, "--prerelease")

	got, err := resolveRelease(t, universal(), rcs, "", true, "stable")
	require.NoError(t, err)
	assert.Equal(t, "v0.7.0-rc.2", got.Tag, "which the flag takes")
}

func TestResolveReleaseAStreamWithNoTagsAtAllSaysNone(t *testing.T) {
	_, err := resolveRelease(t, universal(), nil, "", false, "stable")

	require.ErrorIs(t, err, ErrNoRelease)
	assert.ErrorContains(t, err, "has no release tags")
}
