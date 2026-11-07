package repostatus

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assetsWorkspace is rancher-assets: one branch, its lines listed and its stages
// in CalVer tags.
func assetsWorkspace() *fakeWorkspace {
	return &fakeWorkspace{
		projects: map[string]config.Project{"rancher/rancher-assets": {
			Name: "rancher-assets",
			Repo: config.Repo{Name: "rancher/rancher-assets", ManagedBy: "release-team", DefaultBranch: "main"},
			Release: config.Release{
				Branching: config.BranchingPerLineTag, TagScheme: config.TagSchemeCalverLineStage, TagPattern: "v*",
				RequiresPrerelease: true, Lines: []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}},
			},
		}},
		current: "rancher/rancher-assets",
		paths:   map[string]workspace.RepoPath{"rancher/rancher-assets": {Path: "/src/assets", Source: workspace.PathCwd}},
		tags: []git.Tag{
			tag("v2.16-20261030T1111Z-dev"), tag("v2.16-20261101T0900Z-dev"), tag("v2.16-20261029T0000Z"),
			tag("v2.15-20260915T1243Z"), tag("v2.15-20260915T1243Z-dev"), tag("v2.14-20260401T1000Z"), tag("main-abc1234"),
		},
		branches: []git.Branch{{Name: "main", Commit: "sha-head-main"}},
	}
}

func TestStatusOfAProjectWithItsLinesInTagsHasARowForEachLineOnItsOneBranch(t *testing.T) {
	got, err := gather(context.Background(), assetsWorkspace(), opener(nil, assert.AnError), nil)

	require.NoError(t, err)
	require.Len(t, got.Targets, 2, "the lines it lists, and not v2.14, which has a tag")
	v216, v215 := got.Targets[0], got.Targets[1]
	assert.Equal(t, "v2.16", v216.Line)
	assert.Equal(t, "main", v216.Branch)
	assert.True(t, v216.IsDefaultBranch)
	assert.Equal(t, "sha-head-main", v216.Head)
	assert.Equal(t, "v2.15", v215.Line)
	assert.Equal(t, "main", v215.Branch)
}

func TestStatusOfAProjectWithItsLinesInTagsShowsTheNewestReleaseAndDevTagOfEachLine(t *testing.T) {
	got, err := gather(context.Background(), assetsWorkspace(), opener(nil, assert.AnError), nil)
	require.NoError(t, err)

	v216, v215 := got.Targets[0], got.Targets[1]
	assert.Equal(t, info("v2.16-20261029T0000Z"), v216.LatestStable, "the release stage is the stable one")
	assert.Equal(t, info("v2.16-20261101T0900Z-dev"), v216.LatestPrerelease, "a dev tag newer than it")
	assert.Equal(t, info("v2.15-20260915T1243Z"), v215.LatestStable)
	assert.Nil(t, v215.LatestPrerelease, "its dev tag is the same commit as the release")
}

func TestStatusOfAProjectWithItsLinesInTagsHasNoStateAndNoNextTag(t *testing.T) {
	got, err := gather(context.Background(), assetsWorkspace(), opener(nil, assert.AnError), nil)
	require.NoError(t, err)

	for _, target := range got.Targets {
		assert.Empty(t, target.State, "a tag a person makes has a next one; these are made by CI")
		assert.Empty(t, target.Next)
	}
}
