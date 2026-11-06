package repostatus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	projects   map[string]config.Project
	current    string
	projectErr error
	paths      map[string]workspace.RepoPath
	tags       []git.Tag
	branches   []git.Branch
	refsErr    error
	asked      string
	refCalls   []string
}

func (f *fakeWorkspace) Project(slug string) (config.Project, error) {
	f.asked = slug
	if f.projectErr != nil {
		return config.Project{}, f.projectErr
	}
	if slug == "" {
		slug = f.current
	}
	return f.projects[slug], nil
}

func (f *fakeWorkspace) RepoPath(slug string) (workspace.RepoPath, error) {
	p := f.paths[slug]
	p.Slug = slug
	return p, p.Err
}

func (f *fakeWorkspace) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	f.refCalls = append(f.refCalls, slug)
	return git.Refs{Tags: f.tags, Branches: f.branches}, f.refsErr
}

// fakeClone has local branches, and counts commits from a table keyed
// "from..to". A pair not in the table is unknown to the clone.
type fakeClone struct {
	branches map[string]string
	counts   map[string]int
}

func (f fakeClone) BranchCommit(branch string) (string, bool, error) {
	c, ok := f.branches[branch]
	return c, ok, nil
}

func (f fakeClone) Count(from, to string) (int, bool, error) {
	n, ok := f.counts[from+".."+to]
	return n, ok, nil
}

func opener(c Clone, err error) Opener {
	return func(string) (Clone, error) { return c, err }
}

func rancherProject() config.Project {
	return config.Project{
		Name: "rancher",
		Repo: config.Repo{Name: "rancher/rancher", Owned: true, DefaultBranch: "main"},
		Release: config.Release{
			Branches: "perLine", TagPattern: "v*", RequiresPrerelease: true,
			Lines: []config.ProjectLine{
				{Line: "v2.13"}, // a development line has no branch of its own
				{Line: "v2.12", Version: "v2.12", Branch: "release/v2.12"},
			},
		},
	}
}

func tag(name string) git.Tag { return git.Tag{Name: name, Commit: "sha-" + name} }

func info(name string) *TagInfo { return &TagInfo{Tag: name, Commit: "sha-" + name} }

func fixture() *fakeWorkspace {
	return &fakeWorkspace{
		projects: map[string]config.Project{"rancher/rancher": rancherProject()},
		current:  "rancher/rancher",
		paths:    map[string]workspace.RepoPath{"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathCwd}},
		tags: []git.Tag{
			tag("main-abc1234"), tag("v2.13.0-rc.1"), tag("v2.12.1"), tag("v2.12.2-rc.1"),
			tag("v2.12.0"), tag("v2.13.0-rc.10"), tag("v2.13.0-rc.2"),
		},
		branches: []git.Branch{
			{Name: "main", Commit: "sha-head-main"},
			{Name: "release/v2.12", Commit: "sha-head-release/v2.12"},
			{Name: "release/v2.11", Commit: "sha-head-release/v2.11"}, // not a release branch of this project
		},
	}
}

// syncedClone is a clone whose branches are the host's heads, with counts.
func syncedClone() fakeClone {
	return fakeClone{
		branches: map[string]string{"main": "sha-head-main", "release/v2.12": "sha-head-release/v2.12"},
		counts: map[string]int{
			"sha-v2.13.0-rc.10..sha-head-main":         7,
			"sha-v2.12.2-rc.1..sha-head-release/v2.12": 0,
		},
	}
}

var ctx = context.Background()

// at is the i-th target, failing the test, and not panicking, when there is none.
func at(t *testing.T, st Status, i int) Target {
	t.Helper()
	require.Greater(t, len(st.Targets), i, "targets")
	return st.Targets[i]
}

func byLine(st Status) map[string]Target {
	m := map[string]Target{}
	for _, t := range st.Targets {
		m[t.Line] = t
	}
	return m
}

func TestStatusReadsTheHostOnce(t *testing.T) {
	ws := fixture()
	_, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", ws.asked)
	assert.Equal(t, []string{"rancher/rancher"}, ws.refCalls)
}

func TestStatusLatestTagsPerLine(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	lines := byLine(got)
	assert.Nil(t, lines["v2.13"].LatestStable, "nothing stable in v2.13 yet")
	assert.Equal(t, info("v2.13.0-rc.10"), lines["v2.13"].LatestPrerelease, "rc.10 is after rc.2")
	assert.Equal(t, info("v2.12.1"), lines["v2.12"].LatestStable)
	assert.Equal(t, info("v2.12.2-rc.1"), lines["v2.12"].LatestPrerelease, "newer than v2.12.1, so a release is coming")
}

func TestStatusLinesAndHeads(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2)
	assert.Equal(t, "main", at(t, got, 0).Branch)
	assert.True(t, at(t, got, 0).IsDefaultBranch, "a line with no branch is on the default branch")
	assert.Equal(t, "sha-head-main", at(t, got, 0).Head)
	assert.Equal(t, "release/v2.12", at(t, got, 1).Branch)
	assert.Equal(t, "sha-head-release/v2.12", at(t, got, 1).Head)
}

func TestStatusAMissingBranchHasNoHead(t *testing.T) {
	ws := fixture()
	ws.branches = []git.Branch{{Name: "main", Commit: "sha-head-main"}}
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Empty(t, at(t, got, 1).Head)
	assert.Nil(t, at(t, got, 1).Unreleased)
	assert.Empty(t, at(t, got, 1).Local, "nothing to compare with")
	assert.Contains(t, renderText(got), "(not found)")
}

func TestStatusUnreleasedCommitsSinceTheLatestTag(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	lines := byLine(got)
	require.NotNil(t, lines["v2.13"].Unreleased)
	assert.Equal(t, 7, *lines["v2.13"].Unreleased, "counted from the pre-release, the newest tag")
	require.NotNil(t, lines["v2.12"].Unreleased)
	assert.Zero(t, *lines["v2.12"].Unreleased)
}

func TestStatusUnreleasedCountsFromTheStableTagWhenThereIsNoNewerPrerelease(t *testing.T) {
	ws := fixture()
	ws.tags = []git.Tag{tag("v2.12.1")}
	c := syncedClone()
	c.counts = map[string]int{"sha-v2.12.1..sha-head-release/v2.12": 3}
	got, err := gather(ctx, ws, opener(c, nil), nil)
	require.NoError(t, err)
	require.NotNil(t, byLine(got)["v2.12"].Unreleased)
	assert.Equal(t, 3, *byLine(got)["v2.12"].Unreleased)
}

func TestStatusUnreleasedIsUnknownWhenTheCloneLacksTheCommit(t *testing.T) {
	c := syncedClone()
	c.counts = nil
	got, err := gather(ctx, fixture(), opener(c, nil), nil)
	require.NoError(t, err)
	assert.Nil(t, byLine(got)["v2.13"].Unreleased, "unknown is not zero")
	assert.Contains(t, renderText(got), "-")
}

func TestStatusUnreleasedIsUnknownWithoutATag(t *testing.T) {
	ws := fixture()
	ws.tags = nil
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2, "a row per line, even with nothing to report")
	for _, tg := range got.Targets {
		assert.Nil(t, tg.Unreleased)
	}
}

func TestStatusLocalAgainstTheHost(t *testing.T) {
	tests := map[string]struct {
		clone fakeClone
		want  string
	}{
		"in sync":   {syncedClone(), "in sync"},
		"no branch": {fakeClone{}, "no local branch"},
		"behind": {fakeClone{
			branches: map[string]string{"main": "old"},
			counts:   map[string]int{"old..sha-head-main": 4, "sha-head-main..old": 0},
		}, "behind 4"},
		"ahead": {fakeClone{
			branches: map[string]string{"main": "mine"},
			counts:   map[string]int{"mine..sha-head-main": 0, "sha-head-main..mine": 2},
		}, "ahead 2"},
		"diverged": {fakeClone{
			branches: map[string]string{"main": "mine"},
			counts:   map[string]int{"mine..sha-head-main": 3, "sha-head-main..mine": 2},
		}, "diverged (ahead 2, behind 3)"},
		"head not fetched": {fakeClone{branches: map[string]string{"main": "mine"}}, "behind (fetch to see by how much)"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := gather(ctx, fixture(), opener(tc.clone, nil), nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, at(t, got, 0).Local)
		})
	}
}

func TestStatusTagsFollowTheTagPattern(t *testing.T) {
	ws := fixture()
	p := ws.projects["rancher/rancher"]
	p.Release.TagPattern = "other-*"
	ws.projects["rancher/rancher"] = p
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2, "a row per line, even with nothing to report")
	for _, tg := range got.Targets {
		assert.Nil(t, tg.LatestStable)
		assert.Nil(t, tg.LatestPrerelease)
	}
}

func TestStatusSingleBranchProject(t *testing.T) {
	ws := fixture()
	p := ws.projects["rancher/rancher"]
	p.Release.Branches, p.Release.Lines = "single", nil
	ws.projects["rancher/rancher"] = p
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "main", at(t, got, 0).Branch)
	assert.Empty(t, at(t, got, 0).Line)
	assert.Equal(t, "sha-head-main", at(t, got, 0).Head)
	assert.Equal(t, info("v2.12.1"), at(t, got, 0).LatestStable, "every tag counts when there are no lines")
	out := renderText(got)
	assert.NotContains(t, out, "LINE")
	assert.Contains(t, out, "BRANCH")
}

func TestStatusHostErrorKeepsTheRest(t *testing.T) {
	ws := fixture()
	ws.refsErr = errors.New("network down")
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "network down", got.Error)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Len(t, got.Targets, 2, "the lines come from config")
	assert.Contains(t, renderText(got), "unavailable: network down")
}

func TestStatusMissingCloneKeepsTheRest(t *testing.T) {
	ws := fixture()
	ws.paths["rancher/rancher"] = workspace.RepoPath{Err: workspace.ErrRepoNotFound}
	got, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, workspace.ErrRepoNotFound.Error(), got.Clone)
	assert.Equal(t, info("v2.12.1"), byLine(got)["v2.12"].LatestStable, "the host does not need a clone")
	assert.Equal(t, "sha-head-main", at(t, got, 0).Head)
	assert.Nil(t, at(t, got, 0).Unreleased)
	assert.Contains(t, renderText(got), "Local clone: unavailable")
}

func TestStatusUnreadableClone(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(nil, errors.New("boom")), nil)
	require.NoError(t, err)
	assert.Equal(t, "boom", got.Clone)
}

func TestStatusProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		ws := fixture()
		ws.projectErr = want
		_, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
		assert.ErrorIs(t, err, want)
	}
}

func TestStatusFromCurrentRepository(t *testing.T) {
	ws := fixture()
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Empty(t, ws.asked, "an empty slug asks for the working directory's Project")
	assert.Equal(t, "rancher/rancher", got.Slug)
}

func TestRenderTextTable(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	out := renderText(got)
	for _, want := range []string{
		"rancher/rancher\n", "LINE", "BRANCH", "HEAD", "LATEST STABLE", "LATEST PRE-RELEASE", "UNRELEASED", "LOCAL",
		"v2.13", "main (default branch)", "release/v2.12", "sha-hea", "in sync", "v2.12.1 (sha-v2.", "7",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRenderTagText(t *testing.T) {
	assert.Equal(t, "v1.0.0 (0123456)", tagText(&TagInfo{Tag: "v1.0.0", Commit: "0123456789abcdef"}))
	assert.Equal(t, "-", tagText(nil))
}

func TestRenderJSON(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	out, err := renderJSON(got)
	require.NoError(t, err)
	var back Status
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, got, back)
	assert.Contains(t, out, `"commit": "sha-v2.12.1"`, "JSON carries the full commit")
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	s := clicfg.Settings{Output: clicfg.OutputEnv}
	_, err := Run(ctx, s, t.TempDir(), nil)
	assert.ErrorContains(t, err, "not supported by repo status")
}
