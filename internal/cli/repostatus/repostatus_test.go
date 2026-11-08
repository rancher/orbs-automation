package repostatus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
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
	// The rest is for tracing consumers and sources.
	refsBySlug map[string]git.Refs // used for a slug that is in it, else tags and branches
	engine     *updatedispatcher.Engine
	engineErr  error
	files      map[string]string // "slug@ref:path"
	fileCalls  []string
}

func (f *fakeWorkspace) Updates() (*updatedispatcher.Engine, error) { return f.engine, f.engineErr }

func (f *fakeWorkspace) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	f.fileCalls = append(f.fileCalls, slug+"@"+ref+":"+path)
	content, ok := f.files[slug+"@"+ref+":"+path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(content), nil
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
	if refs, ok := f.refsBySlug[slug]; ok {
		return refs, f.refsErr
	}
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
			Branching: "perLine", TagPattern: "v*", RequiresPrerelease: true,
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
	assert.Contains(t, renderText(got, nil), "(not found)")
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
	assert.Contains(t, renderText(got, nil), "-")
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
	p.Release.Branching, p.Release.Lines = "single", nil
	ws.projects["rancher/rancher"] = p
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, "main", at(t, got, 0).Branch)
	assert.Empty(t, at(t, got, 0).Line)
	assert.Equal(t, "sha-head-main", at(t, got, 0).Head)
	assert.Equal(t, info("v2.12.1"), at(t, got, 0).LatestStable, "every tag counts when there are no lines")
	out := renderText(got, nil)
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
	assert.Contains(t, renderText(got, nil), "unavailable: network down")
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
	assert.Contains(t, renderText(got, nil), "Local clone: unavailable")
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
	out := renderText(got, nil)
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
	_, err := Run(ctx, s, t.TempDir(), nil, Options{})
	assert.ErrorContains(t, err, "not supported by repo status")
}

// nextTags are the tags of a target's candidates, in order.
func nextTags(t Target) []string {
	var out []string
	for _, n := range t.Next {
		out = append(out, n.Tag)
	}
	return out
}

func TestStatusAnRcAndTheBranchPastItCanHaveAnotherRcButNotAPromotion(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	v213 := byLine(got)["v2.13"]

	assert.Equal(t, "rc-open-work", v213.State)
	require.Len(t, v213.Next, 2)
	assert.Equal(t, Next{Tag: "v2.13.0-rc.11", Kind: "prerelease", Available: true}, v213.Next[0])
	assert.Equal(t, Next{Tag: "v2.13.0", Kind: "promotion", Promotes: "v2.13.0-rc.10", Available: false, Why: "the branch has moved since the rc"}, v213.Next[1])
}

func TestStatusAnRcOnTheHeadCanBePromotedAndAnotherRcSaysNothingIsNew(t *testing.T) {
	ws := fixture()
	ws.branches[0].Commit = "sha-v2.13.0-rc.10" // main's head is the rc

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v213 := byLine(got)["v2.13"]
	assert.Equal(t, "rc-open", v213.State, "the host alone says so, whatever the clone has")
	require.Len(t, v213.Next, 2)
	assert.Equal(t, Next{Tag: "v2.13.0-rc.11", Kind: "prerelease", Available: false, Why: "no new commits"}, v213.Next[0])
	assert.Equal(t, Next{Tag: "v2.13.0", Kind: "promotion", Promotes: "v2.13.0-rc.10", Available: true}, v213.Next[1])
}

func TestStatusAStableTagOnTheHeadIsReleasedAndHasNothingNext(t *testing.T) {
	ws := fixture()
	ws.tags = []git.Tag{tag("v2.12.1")}
	ws.branches[1].Commit = "sha-v2.12.1"

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "released", v212.State)
	assert.Empty(t, v212.Next)
}

func TestStatusAStableTagWithWorkSinceItNamesTheNextPrerelease(t *testing.T) {
	ws := fixture()
	ws.tags = []git.Tag{tag("v2.12.1")}

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "work-since-release", v212.State)
	require.Equal(t, []string{"v2.12.2-rc.1"}, nextTags(v212))
	assert.True(t, v212.Next[0].Available)
}

func TestStatusALineWithNoTagStartsAtItsSeries(t *testing.T) {
	ws := fixture()
	ws.tags = nil

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "no-release", v212.State)
	assert.Equal(t, []string{"v2.12.0-rc.1"}, nextTags(v212))
}

func TestStatusABranchTheHostDoesNotHaveHasNoStateAndNothingNext(t *testing.T) {
	ws := fixture()
	ws.branches = ws.branches[:1]

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Empty(t, v212.State)
	assert.Empty(t, v212.Next)
}

func TestRenderTextHasAReleasesSectionWithStateAndNextAndALocalCloneSection(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)
	require.NoError(t, err)

	out := renderText(got, nil)

	releases, local, found := strings.Cut(out, "\nLocal clone\n")
	require.True(t, found, out)
	assert.Contains(t, releases, "\nReleases\n")
	for _, want := range []string{"LATEST STABLE", "LATEST PRE-RELEASE", "STATE", "NEXT", "rc open, work since the rc",
		"v2.13.0-rc.11", "(v2.13.0 promotes v2.13.0-rc.10: the branch has moved since the rc)"} {
		assert.Contains(t, releases, want)
	}
	assert.NotContains(t, releases, "UNRELEASED", "clone facts are their own section")
	assert.NotContains(t, releases, "LOCAL")
	for _, want := range []string{"UNRELEASED", "LOCAL", "in sync", "7"} {
		assert.Contains(t, local, want)
	}
}

func TestRenderTextAPromotionThatIsAvailableIsNotInParentheses(t *testing.T) {
	ws := fixture()
	ws.branches[0].Commit = "sha-v2.13.0-rc.10"
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)

	out := renderText(got, nil)

	assert.Contains(t, out, "v2.13.0 (promotes v2.13.0-rc.10)")
	assert.Contains(t, out, "(v2.13.0-rc.11: no new commits)")
	assert.Contains(t, out, "rc open, head on the rc")
}

// sccOperator is a producer with no release lines, shipping to rancher.
func sccOperator() config.Project {
	return config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator", Owned: true, DefaultBranch: "main"},
		Release: config.Release{Branching: "single", TagPattern: "v*"},
		Downstream: []config.Edge{{
			To: "rancher", Method: "build-yaml-bump",
			Params: map[string]string{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"},
		}},
	}
}

// shipping makes the fixture a project that takes from scc-operator, and
// scc-operator one that ships to it: both projects, the engine of their update
// methods, the refs of each and the file rancher holds the value in.
func shipping(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := fixture()
	rancher := rancherProject()
	rancher.UpdateMethods = []config.UpdateMethod{{
		Name: "build-yaml-bump", Strategy: "yamlBump", With: map[string]string{"file": "build.yaml"},
		AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
	}}
	ws.projects = map[string]config.Project{"rancher/rancher": rancher, "rancher/scc-operator": sccOperator()}
	engine, err := updatedispatcher.NewEngine(updatedispatcher.Builtin(), config.Config{Projects: []config.Project{sccOperator(), rancher}})
	require.NoError(t, err)
	ws.engine = engine
	ws.refsBySlug = map[string]git.Refs{
		"rancher/scc-operator": {
			Tags:     []git.Tag{tag("v0.5.2"), tag("v0.5.3"), tag("v0.6.0-rc.1")},
			Branches: []git.Branch{{Name: "main", Commit: "sha-scc-main"}},
		},
		"rancher/rancher": {Tags: ws.tags, Branches: ws.branches},
	}
	ws.files = map[string]string{
		"rancher/rancher@sha-head-main:build.yaml":          "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n",
		"rancher/rancher@sha-head-release/v2.12:build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n",
	}
	return ws
}

func holdingOn(t *testing.T, hs []ship.Holding, branch string) ship.Holding {
	t.Helper()
	for _, h := range hs {
		if h.Branch == branch {
			return h
		}
	}
	require.Failf(t, "no such holding", "%s in %v", branch, hs)
	return ship.Holding{}
}

func TestStatusOfAConsumerHasASourceForEachProjectThatShipsToIt(t *testing.T) {
	ws := shipping(t)

	got, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"})

	require.NoError(t, err)
	require.Len(t, got.Sources, 1)
	src := got.Sources[0]
	assert.Equal(t, "rancher/scc-operator", src.Producer)
	assert.Equal(t, "build-yaml-bump", src.Method)
	require.Len(t, src.Targets, 2)
	main := holdingOn(t, src.Targets, "main")
	assert.Equal(t, "rancher/scc-operator:v0.5.3", main.Current)
	assert.True(t, main.Stable.UpToDate)
	assert.Equal(t, "v0.6.0-rc.1", main.Newest.Tag)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", holdingOn(t, src.Targets, "release/v2.12").Current)
	assert.Empty(t, got.Consumers, "rancher ships to no one")
}

func TestStatusOfAProducerHasAConsumerForEachProjectItShipsTo(t *testing.T) {
	ws := shipping(t)

	got, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/scc-operator"})

	require.NoError(t, err)
	require.Len(t, got.Consumers, 1)
	c := got.Consumers[0]
	assert.Equal(t, "rancher/rancher", c.Consumer)
	require.Len(t, c.Targets, 2)
	assert.Equal(t, "v2.12", holdingOn(t, c.Targets, "release/v2.12").Line)
	assert.Empty(t, got.Sources, "scc-operator takes from no one")
	assert.Equal(t, "rancher/scc-operator", got.Slug)
}

func TestStatusOfAProjectThatShipsNowhereAndTakesFromNoOneHasNeither(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	assert.Empty(t, got.Consumers)
	assert.Empty(t, got.Sources)
	assert.Empty(t, got.TraceError)
}

func TestStatusOptionsLeaveOutTheConsumersOrTheSourcesAndTheirRequests(t *testing.T) {
	ws := shipping(t)
	got, err := gatherWith(ctx, ws, opener(syncedClone(), nil), []string{"rancher/scc-operator"}, Options{NoConsumers: true})
	require.NoError(t, err)
	assert.Empty(t, got.Consumers)
	assert.Empty(t, ws.fileCalls, "nothing of the consumer was read")

	ws = shipping(t)
	got, err = gatherWith(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"}, Options{NoSources: true})
	require.NoError(t, err)
	assert.Empty(t, got.Sources)
	assert.Empty(t, ws.fileCalls)
}

func TestStatusAnEngineThatCannotBeUsedIsATraceErrorAndKeepsTheRest(t *testing.T) {
	ws := fixture()
	engine, err := updatedispatcher.NewEngine(updatedispatcher.Builtin(), config.Config{})
	require.NoError(t, err)
	ws.engine, ws.engineErr = engine, errors.New("method build-yaml-bump of rancher is not usable")

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	assert.Contains(t, got.TraceError, "not usable")
	assert.Len(t, got.Targets, 2, "the releases are reported all the same")
}

func TestHoldStatus(t *testing.T) {
	stable := func(tag string, upToDate bool, rel updatedispatcher.Relation) *ship.Verdict {
		return &ship.Verdict{Tag: tag, UpToDate: upToDate, Relation: rel}
	}
	const (
		newer = updatedispatcher.RelationNewer
		same  = updatedispatcher.RelationSame
		older = updatedispatcher.RelationOlder
	)
	tests := []struct {
		name string
		h    ship.Holding
		want string
	}{
		{"an error", ship.Holding{Error: "no value for the key"}, "unreadable: no value for the key"},
		{"nothing read", ship.Holding{}, "-"},
		{"on the release, no prerelease newer", ship.Holding{Stable: stable("v0.5.3", true, same), Newest: stable("v0.5.3", true, same)}, "current"},
		{"on the release, no newest at all", ship.Holding{Stable: stable("v0.5.3", true, same)}, "current"},
		{"behind the release", ship.Holding{Stable: stable("v0.5.3", false, newer), Newest: stable("v0.5.3", false, newer)}, "behind v0.5.3"},
		{"ahead of the release", ship.Holding{Stable: stable("v0.5.3", false, older)}, "ahead of v0.5.3"},
		{"a version it cannot compare", ship.Holding{Stable: stable("v0.5.3", false, updatedispatcher.RelationUnknown)}, "unknown"},
		{"on the newest, a prerelease", ship.Holding{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.1", true, same)}, "current"},
		{"on the stable, a prerelease is newer", ship.Holding{Stable: stable("v0.5.3", true, same), Newest: stable("v0.6.0-rc.1", false, newer)}, "on stable, v0.6.0-rc.1 is newer"},
		{"behind the stable, a prerelease is newer", ship.Holding{Stable: stable("v0.5.3", false, newer), Newest: stable("v0.6.0-rc.1", false, newer)}, "behind v0.5.3"},
		{"between the stable and the newest", ship.Holding{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.2", false, newer)}, "on a prerelease, v0.6.0-rc.2 is newer"},
		{"past the newest", ship.Holding{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.1", false, older)}, "ahead of v0.6.0-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, holdStatus(tt.h))
		})
	}
}

func TestRenderTextHasASourcesSectionForWhatTheProjectTakes(t *testing.T) {
	got, err := gather(ctx, shipping(t), opener(syncedClone(), nil), []string{"rancher/rancher"})
	require.NoError(t, err)

	out := renderText(got, nil)

	_, sources, found := strings.Cut(out, "\nSources: scc-operator (rancher/scc-operator), build-yaml-bump\n")
	require.True(t, found, out)
	for _, want := range []string{"LINE", "BRANCH", "HEAD", "HOLDS", "LATEST STABLE", "STATUS",
		"rancher/scc-operator:v0.5.3", "v0.5.3", "on stable, v0.6.0-rc.1 is newer", "behind v0.5.3", "release/v2.12"} {
		assert.Contains(t, sources, want)
	}
	assert.NotContains(t, out, "Consumers:")
}

func TestRenderTextHasAConsumersSectionForWhatTheProjectShips(t *testing.T) {
	got, err := gather(ctx, shipping(t), opener(syncedClone(), nil), []string{"rancher/scc-operator"})
	require.NoError(t, err)

	out := renderText(got, nil)

	assert.Contains(t, out, "\nConsumers: rancher (rancher/rancher), build-yaml-bump\n")
	assert.NotContains(t, out, "Sources:")
}

func TestRenderTextAConsumerThatCouldNotBeTracedSaysWhy(t *testing.T) {
	st := Status{Slug: "rancher/scc-operator", Consumers: []ship.ConsumerHoldings{
		{Consumer: "rancher/rancher", Name: "rancher", Method: "build-yaml-bump", Error: "read the branches of rancher/rancher: network down"},
	}, TraceError: "method x is not usable"}

	out := renderText(st, nil)

	assert.Contains(t, out, "Consumers: rancher (rancher/rancher), build-yaml-bump\n  unavailable: read the branches of rancher/rancher: network down")
	assert.Contains(t, out, "could not trace: method x is not usable")
}

func TestRenderJSONCarriesTheConsumersAndTheSources(t *testing.T) {
	got, err := gather(ctx, shipping(t), opener(syncedClone(), nil), []string{"rancher/rancher"})
	require.NoError(t, err)

	out, err := renderJSON(got)

	require.NoError(t, err)
	var back Status
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, got, back)
	assert.Contains(t, out, `"sources"`)
	assert.Contains(t, out, `"current": "rancher/scc-operator:v0.5.3"`)
}
