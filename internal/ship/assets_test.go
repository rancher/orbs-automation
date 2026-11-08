package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assetsFixture is rancher-assets as the config has it: not owned, one branch, its
// lines and stages in CalVer tags, and an edge to Rancher's build-yaml-bump for
// defaultAssetsImage. The project lists v2.16 and v2.15; v2.14 has a tag and a
// Rancher branch with no such key. Rancher holds a dev tag on v2.16's branch and a
// release tag on v2.15's.
func assetsFixture() *fakeWorkspace {
	producer := config.Project{
		Name: "rancher-assets", Repo: config.Repo{Name: "rancher/rancher-assets", ManagedBy: "release-team", DefaultBranch: "main"},
		Release: config.Release{
			Branching: config.BranchingPerLineTag, TagScheme: config.TagSchemeCalverLineStage, TagPattern: "v*",
			Lines: []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}},
		},
		Images: []config.Image{{Name: "rancher/rancher-assets"}},
	}
	l := riLink("yamlBump")
	l.Consumer.Release.Lines = []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15", Branch: "release/v2.15"}, {Line: "v2.14", Branch: "release/v2.14"}}
	l.Edge.Params = map[string]string{"updateKey": "defaultAssetsImage", "value": "{image}:{tag}"}
	return &fakeWorkspace{
		producer: producer,
		links:    []link{l},
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
	got, err := holdings(t, assetsFixture())

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
	got, err := holdings(t, assetsFixture())
	require.NoError(t, err)
	hs := byBranch(t, got[0])

	main := hs["main"]
	require.NotNil(t, main.Stable)
	assert.Equal(t, Verdict{
		Tag: "v2.16-20261029T0000Z", Want: "rancher/rancher-assets:v2.16-20261029T0000Z",
		UpToDate: false, Relation: updatedispatcher.RelationOlder,
	}, *main.Stable, "the newest release tag is older than the dev tag Rancher holds")
	require.NotNil(t, main.Newest)
	assert.Equal(t, "v2.16-20261101T0900Z-dev", main.Newest.Tag)
	assert.Equal(t, updatedispatcher.RelationNewer, main.Newest.Relation, "a newer date with the same line is a newer image")

	v215 := hs["release/v2.15"]
	require.NotNil(t, v215.Stable)
	assert.True(t, v215.Stable.UpToDate)
	assert.Equal(t, updatedispatcher.RelationSame, v215.Stable.Relation)
	require.NotNil(t, v215.Newest)
	assert.Equal(t, *v215.Stable, *v215.Newest, "no dev tag is newer than the release")
}

func TestHoldingsOfAnAssetsProducerSayWhichStageIsHeld(t *testing.T) {
	got, err := holdings(t, assetsFixture())
	require.NoError(t, err)
	hs := byBranch(t, got[0])

	assert.Equal(t, "dev", hs["main"].Stage)
	assert.Equal(t, "stable", hs["release/v2.15"].Stage)
}

func TestSourcesOfRancherShowTheAssetsTagItHolds(t *testing.T) {
	ws := assetsFixture()
	producer, err := ws.Project("")
	require.NoError(t, err)
	ws.producer = producer

	engine, engineErr := ws.Updates()
	require.NoError(t, engineErr)
	consumer := ws.links[0].Consumer

	got := engine.RoutesInto(consumer)
	require.Len(t, got, 1)

	srcs, err := Sources(ctx, ws, consumer)

	require.NoError(t, err)
	require.Len(t, srcs, 1)
	assert.Equal(t, "rancher/rancher-assets", srcs[0].Producer)
	assert.Empty(t, srcs[0].Error)
	assert.Len(t, srcs[0].Targets, 2)
}

func TestAnAssetsProducerHasNoReleaseToShip(t *testing.T) {
	_, err := Check(ctx, assetsFixture(), &fakeProber{}, Input{Project: "rancher-assets"})

	require.ErrorIs(t, err, ErrNotOwned)
}
