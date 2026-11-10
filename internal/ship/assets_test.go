package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assetsFixture is rancher-assets as the config has it: not owned, one branch, its
// lines and stages in CalVer tags, and an edge to Rancher's build-yaml-bump for
// defaultAssetsImage. The project lists v2.16 and v2.15; v2.14 has a tag and a
// Rancher branch with no such key. Rancher holds a dev tag on v2.16's branch and a
// release tag on v2.15's.
func assetsFixture(t *testing.T) *fakeWorkspace {
	t.Helper()
	l := riLink()
	l.Consumer.Lines = []lineSpec{{Line: "v2.16"}, {Line: "v2.15", Branch: "release/v2.15"}, {Line: "v2.14", Branch: "release/v2.14"}}
	l.Key = "defaultAssetsImage"
	return &fakeWorkspace{
		t: t,
		producer: projectSpec{
			Name: "rancher-assets", Repo: "rancher/rancher-assets", ManagedBy: "release-team", Images: []string{"rancher/rancher-assets"},
			Scheme: "calverLineStage", Stream: "  lines: [{line: v2.16}, {line: v2.15}]\n",
		},
		links: []link{l},
		refs: map[string]git.Refs{
			"rancher/rancher-assets": {Tags: tags(
				"v2.16-20261030T1111Z-dev", "v2.16-20261101T0900Z-dev", "v2.16-20261029T0000Z",
				"v2.15-20260915T1243Z", "v2.15-20260915T1243Z-dev", "v2.14-20260401T1000Z", "main-abc1234",
			)},
			"rancher/rancher": {Branches: []git.Branch{{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"}, {Name: "release/v2.14", Commit: "sha-214"}}},
		},
		files: map[string]string{
			"rancher/rancher@sha-main:build.yaml": "defaultAssetsImage: rancher/rancher-assets:v2.16-20261030T1111Z-dev\n",
			"rancher/rancher@sha-215:build.yaml":  "defaultAssetsImage: rancher/rancher-assets:v2.15-20260915T1243Z\n",
		},
	}
}

func TestHoldingsOfAnAssetsProducerFollowTheLinesItListsThatRancherHas(t *testing.T) {
	got, err := holdings(t, assetsFixture(t))

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Empty(t, got[0].Error)
	hs := byBranch(t, got[0])
	require.Len(t, hs, 2, "the lines both list: v2.14 is in Rancher's lines and has tags, and Assets is not used for it")
	assert.Equal(t, "v2.16", hs["main"].Line)
	assert.Equal(t, "v2.15", hs["release/v2.15"].Line)
	assert.Equal(t, "rancher/rancher-assets:v2.16-20261030T1111Z-dev", hs["main"].Current)
}

func TestHoldingsOfAnAssetsProducerTakeTheReleaseStageAsStableAndTheNewerDevTagAsNewest(t *testing.T) {
	got, err := holdings(t, assetsFixture(t))
	require.NoError(t, err)
	hs := byBranch(t, got[0])

	main := hs["main"]
	require.NotNil(t, main.Stable)
	assert.Equal(t, Verdict{
		Tag: "v2.16-20261029T0000Z", Want: "rancher/rancher-assets:v2.16-20261029T0000Z",
		UpToDate: false, Relation: bump.RelationOlder,
	}, *main.Stable, "the newest release tag is older than the dev tag Rancher holds")
	require.NotNil(t, main.Newest)
	assert.Equal(t, "v2.16-20261101T0900Z-dev", main.Newest.Tag)
	assert.Equal(t, bump.RelationNewer, main.Newest.Relation, "a newer date with the same line is a newer image")

	v215 := hs["release/v2.15"]
	require.NotNil(t, v215.Stable)
	assert.True(t, v215.Stable.UpToDate)
	assert.Equal(t, bump.RelationSame, v215.Stable.Relation)
	require.NotNil(t, v215.Newest)
	assert.Equal(t, *v215.Stable, *v215.Newest, "no dev tag is newer than the release")
}

func TestHoldingsOfAnAssetsProducerSayWhichStageIsHeld(t *testing.T) {
	got, err := holdings(t, assetsFixture(t))
	require.NoError(t, err)
	hs := byBranch(t, got[0])

	assert.Equal(t, "dev", hs["main"].Stage)
	assert.Equal(t, "stable", hs["release/v2.15"].Stage)
}

func TestSourcesOfRancherShowTheAssetsTagItHolds(t *testing.T) {
	ws := assetsFixture(t)
	engine, engineErr := ws.Bump()
	require.NoError(t, engineErr)
	consumer := ws.config().Project("rancher")

	require.Len(t, engine.RoutesInto(consumer), 1)

	srcs, err := Sources(ctx, ws, consumer)

	require.NoError(t, err)
	require.Len(t, srcs, 1)
	assert.Equal(t, "rancher/rancher-assets", srcs[0].Producer)
	assert.Empty(t, srcs[0].Error)
	assert.Len(t, srcs[0].Targets, 2)
}

func TestAnAssetsProducerHasNoReleaseToShip(t *testing.T) {
	_, err := Check(ctx, assetsFixture(t), &fakeProber{}, Input{Team: testTeam, Project: "rancher-assets"})

	require.ErrorIs(t, err, ErrNotOwned)
}
