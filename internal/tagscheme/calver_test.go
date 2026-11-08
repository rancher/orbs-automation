package tagscheme

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assets is a project like rancher-assets: one branch, and its lines and stages in
// CalVer tags.
func assets() config.Project {
	return config.Project{
		Name: "rancher-assets", Repo: config.Repo{Name: "rancher/rancher-assets"},
		Release: config.Release{
			Branching: config.BranchingPerLineTag, TagScheme: config.TagSchemeCalverLineStage, TagPattern: "v*",
			Lines: []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}},
		},
	}
}

// assetsTags are the tags as they come: both stages, several lines, a prod tag
// with no dev twin, and names that are not releases.
func assetsTags() []git.Tag {
	return tags(
		"v2.16-20261030T1111Z-dev",
		"v2.16-20261101T0900Z-dev",
		"v2.16-20261029T0000Z",
		"v2.15-20260915T1243Z",
		"v2.15-20260915T1243Z-dev",
		"v2.14-20260401T1000Z",
		"v2.16",
		"main-abc1234",
	)
}

func calverFor(t *testing.T) Scheme {
	t.Helper()
	s, err := For(assets())
	require.NoError(t, err)
	require.Equal(t, "calverLineStage", s.Name())
	return s
}

func TestForAPerLineTagProjectWithAnUnknownSchemeIsAnError(t *testing.T) {
	p := assets()
	p.Release.TagScheme = "semver"

	_, err := For(p)

	require.Error(t, err)
	assert.ErrorContains(t, err, "rancher-assets")
	assert.ErrorContains(t, err, "calverLineStage", "it lists the schemes there are")
}

func TestCalverValid(t *testing.T) {
	s := calverFor(t)
	for tag, want := range map[string]bool{
		"v2.16-20261030T1111Z": true, "v2.16-20261030T1111Z-dev": true,
		"v2.16": false, "v8.1.0": false, "main-abc1234": false, "": false,
	} {
		assert.Equal(t, want, s.Valid(tag), tag)
	}
}

func TestCalverLinesAreTheOnesTheConfigListsWhateverTheTags(t *testing.T) {
	assert.Equal(t, []string{"v2.16", "v2.15"}, calverFor(t).Lines(assetsTags()), "v2.14 has tags and is not listed")
	assert.Equal(t, []string{"v2.16", "v2.15"}, calverFor(t).Lines(nil))
}

func TestCalverLineOf(t *testing.T) {
	s := calverFor(t)
	assert.Equal(t, "v2.15", s.LineOf("v2.15-20260915T1243Z"))
	assert.Equal(t, "v2.16", s.LineOf("v2.16-20261030T1111Z-dev"))
	assert.Equal(t, "", s.LineOf("main-abc1234"))
}

func TestCalverLatestOfALineIsTheNewestReleaseAndTheNewerDevTag(t *testing.T) {
	got := calverFor(t).Latest(assetsTags(), "v2.16")

	require.NotNil(t, got.Stable)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, git.Tag{Name: "v2.16-20261029T0000Z", Commit: "sha-v2.16-20261029T0000Z"}, *got.Stable, "the release stage is the stable one, with its commit")
	assert.Equal(t, "v2.16-20261101T0900Z-dev", got.Prerelease.Name, "the newest dev tag, which is newer than the release")
}

func TestCalverALineWhoseDevTagsAreOlderThanItsReleaseHasNoPrerelease(t *testing.T) {
	got := calverFor(t).Latest(assetsTags(), "v2.15")

	require.NotNil(t, got.Stable)
	assert.Equal(t, "v2.15-20260915T1243Z", got.Stable.Name)
	assert.Nil(t, got.Prerelease, "a dev tag of the same minute is not newer than the release")
}

func TestCalverALineWithOnlyDevTagsHasNoStableAndItsNewestDevAsPrerelease(t *testing.T) {
	got := calverFor(t).Latest(tags("v2.16-20261105T0800Z-dev", "v2.16-20261104T0800Z-dev"), "v2.16")

	assert.Nil(t, got.Stable)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v2.16-20261105T0800Z-dev", got.Prerelease.Name)
}

func TestCalverLatestOfEveryLineIsTheNewestByDate(t *testing.T) {
	got := calverFor(t).Latest(assetsTags(), "")

	require.NotNil(t, got.Stable)
	assert.Equal(t, "v2.16-20261029T0000Z", got.Stable.Name)
	require.NotNil(t, got.Prerelease)
	assert.Equal(t, "v2.16-20261101T0900Z-dev", got.Prerelease.Name)
}

func TestCalverLatestOfALineWithNoTagsOrNotListedIsEmpty(t *testing.T) {
	s := calverFor(t)
	assert.Equal(t, git.Latest{}, s.Latest(assetsTags(), "v2.13"))
	assert.Equal(t, git.Latest{}, s.Latest(nil, "v2.16"))
	assert.Equal(t, git.Latest{}, s.Latest(assetsTags(), "v2.14"), "it has a tag, and the project is not used for the line")
}

func TestCalverLatestOfEveryLineLeavesOutTheLinesNotListed(t *testing.T) {
	got := calverFor(t).Latest(tags("v2.14-20270101T0000Z", "v2.16-20261029T0000Z"), "")

	require.NotNil(t, got.Stable)
	assert.Equal(t, "v2.16-20261029T0000Z", got.Stable.Name, "v2.14 is newer by date and is not a line of the project")
}

func TestCalverIsStableIsTheReleaseStage(t *testing.T) {
	s := calverFor(t)
	assert.True(t, s.IsStable("v2.16-20261029T0000Z"))
	assert.False(t, s.IsStable("v2.16-20261030T1111Z-dev"))
	assert.False(t, s.IsStable("main-abc1234"), "not a release at all")
}

func TestCalverComparesByDate(t *testing.T) {
	s := calverFor(t)
	assert.Positive(t, s.Compare("v2.16-20261101T0900Z-dev", "v2.16-20261030T1111Z-dev"), "a newer date with the same line is newer")
	assert.Negative(t, s.Compare("v2.16-20261029T0000Z", "v2.16-20261030T1111Z-dev"), "across stages too")
	assert.Zero(t, s.Compare("v2.16-20261030T1111Z", "v2.16-20261030T1111Z"))
	assert.Zero(t, s.Compare("v2.16-20261030T1111Z", "main-abc1234"), "a name that is not a release has no order")
}

func TestCalverStageIsTheSuffix(t *testing.T) {
	s := calverFor(t)
	assert.Equal(t, "dev", s.Stage("v2.16-20261030T1111Z-dev"))
	assert.Equal(t, "stable", s.Stage("v2.16-20261029T0000Z"))
	assert.Equal(t, "", s.Stage("main-abc1234"))
}

func TestSemverTagsCarryNoStage(t *testing.T) {
	assert.Equal(t, "", semverFor(t, lined()).Stage("v8.1.0-rc.1"))
}
