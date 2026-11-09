package repostatus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	t          *testing.T
	docs       []string // the config, as documents
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
	engine     *bump.Engine        // used when set, with engineErr, in place of the real one
	engineErr  error
	files      map[string]string // "slug@ref:path"
	fileCalls  []string

	graph    *chain.Graph
	graphKey string
}

// config joins the documents, again when they have changed since.
func (f *fakeWorkspace) config() *chain.Graph {
	if key := strings.Join(f.docs, "\x00"); f.graph == nil || key != f.graphKey {
		f.graph, f.graphKey = chaintest.Load(f.t, f.docs...), key
	}
	return f.graph
}

func (f *fakeWorkspace) Bump() (*bump.Engine, error) {
	if f.engine != nil || f.engineErr != nil {
		return f.engine, f.engineErr
	}
	return bump.NewEngine(bump.Builtin(), f.config())
}

func (f *fakeWorkspace) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	f.fileCalls = append(f.fileCalls, slug+"@"+ref+":"+path)
	content, ok := f.files[slug+"@"+ref+":"+path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(content), nil
}

func (f *fakeWorkspace) Project(key string) (*chain.Project, error) {
	f.asked = key
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	if key == "" {
		key = f.current
	}
	return f.config().Project(key), nil
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

// rancherDocs is rancher: a branch for each of two lines, the development line on
// main, and one stream with a series for each, with the given extra lines of the
// stream's spec.
func rancherDocs(streamExtra string) []string {
	return []string{
		chaintest.Lines("v2.13", "v2.12"),
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher, owned: true}\n  branches:\n    layout: perLine\n    lines:\n      - {line: v2.13}\n      - {line: v2.12, branch: release/v2.12}\n"),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  requiresPrerelease: true\n"+streamExtra+"  lines:\n    - {line: v2.13, series: \"2.13.x\"}\n    - {line: v2.12, series: \"2.12.x\"}\n"),
	}
}

func tag(name string) git.Tag { return git.Tag{Name: name, Commit: "sha-" + name} }

func info(name string) *TagInfo { return &TagInfo{Tag: name, Commit: "sha-" + name} }

func fixture(t *testing.T) *fakeWorkspace {
	t.Helper()
	return &fakeWorkspace{
		t:       t,
		docs:    rancherDocs(""),
		current: "rancher/rancher",
		paths:   map[string]workspace.RepoPath{"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathCwd}},
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
	ws := fixture(t)
	_, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", ws.asked)
	assert.Equal(t, []string{"rancher/rancher"}, ws.refCalls)
}

func TestStatusLatestTagsPerLine(t *testing.T) {
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	lines := byLine(got)
	assert.Nil(t, lines["v2.13"].LatestStable, "nothing stable in v2.13 yet")
	assert.Equal(t, info("v2.13.0-rc.10"), lines["v2.13"].LatestPrerelease, "rc.10 is after rc.2")
	assert.Equal(t, info("v2.12.1"), lines["v2.12"].LatestStable)
	assert.Equal(t, info("v2.12.2-rc.1"), lines["v2.12"].LatestPrerelease, "newer than v2.12.1, so a release is coming")
}

func TestStatusLinesAndHeads(t *testing.T) {
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2)
	assert.Equal(t, "main", at(t, got, 0).Branch)
	assert.True(t, at(t, got, 0).IsDefaultBranch, "a line with no branch is on the default branch")
	assert.Equal(t, "sha-head-main", at(t, got, 0).Head)
	assert.Equal(t, "release/v2.12", at(t, got, 1).Branch)
	assert.Equal(t, "sha-head-release/v2.12", at(t, got, 1).Head)
}

func TestStatusAMissingBranchHasNoHead(t *testing.T) {
	ws := fixture(t)
	ws.branches = []git.Branch{{Name: "main", Commit: "sha-head-main"}}
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Empty(t, at(t, got, 1).Head)
	assert.Nil(t, at(t, got, 1).Unreleased)
	assert.Empty(t, at(t, got, 1).Local, "nothing to compare with")
	assert.Contains(t, renderText(got, nil), "(not found)")
}

func TestStatusUnreleasedCommitsSinceTheLatestTag(t *testing.T) {
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	lines := byLine(got)
	require.NotNil(t, lines["v2.13"].Unreleased)
	assert.Equal(t, 7, *lines["v2.13"].Unreleased, "counted from the pre-release, the newest tag")
	require.NotNil(t, lines["v2.12"].Unreleased)
	assert.Zero(t, *lines["v2.12"].Unreleased)
}

func TestStatusUnreleasedCountsFromTheStableTagWhenThereIsNoNewerPrerelease(t *testing.T) {
	ws := fixture(t)
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
	got, err := gather(ctx, fixture(t), opener(c, nil), nil)
	require.NoError(t, err)
	assert.Nil(t, byLine(got)["v2.13"].Unreleased, "unknown is not zero")
	assert.Contains(t, renderText(got, nil), "-")
}

func TestStatusUnreleasedIsUnknownWithoutATag(t *testing.T) {
	ws := fixture(t)
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
			got, err := gather(ctx, fixture(t), opener(tc.clone, nil), nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, at(t, got, 0).Local)
		})
	}
}

func TestStatusTagsFollowTheTagPattern(t *testing.T) {
	ws := fixture(t)
	ws.docs = rancherDocs("  tagPattern: \"other-*\"\n")
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2, "a row per line, even with nothing to report")
	for _, tg := range got.Targets {
		assert.Nil(t, tg.LatestStable)
		assert.Nil(t, tg.LatestPrerelease)
	}
}

func TestStatusSingleBranchProject(t *testing.T) {
	ws := fixture(t)
	ws.docs = []string{
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher, owned: true}\n"),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  universal: true\n"),
	}
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
	ws := fixture(t)
	ws.refsErr = errors.New("network down")
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "network down", got.Error)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Len(t, got.Targets, 2, "the lines come from config")
	assert.Contains(t, renderText(got, nil), "unavailable: network down")
}

func TestStatusMissingCloneKeepsTheRest(t *testing.T) {
	ws := fixture(t)
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
	got, err := gather(ctx, fixture(t), opener(nil, errors.New("boom")), nil)
	require.NoError(t, err)
	assert.Equal(t, "boom", got.Clone)
}

func TestStatusProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		ws := fixture(t)
		ws.projectErr = want
		_, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
		assert.ErrorIs(t, err, want)
	}
}

func TestStatusFromCurrentRepository(t *testing.T) {
	ws := fixture(t)
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	assert.Empty(t, ws.asked, "an empty slug asks for the working directory's Project")
	assert.Equal(t, "rancher/rancher", got.Slug)
}

func TestRenderTextTable(t *testing.T) {
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
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
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
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
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
	require.NoError(t, err)
	v213 := byLine(got)["v2.13"]

	assert.Equal(t, "rc-open-work", v213.State)
	require.Len(t, v213.Next, 2)
	assert.Equal(t, Next{Tag: "v2.13.0-rc.11", Kind: "prerelease", Available: true}, v213.Next[0])
	assert.Equal(t, Next{Tag: "v2.13.0", Kind: "promotion", Promotes: "v2.13.0-rc.10", Available: false, Why: "the branch has moved since the rc"}, v213.Next[1])
}

func TestStatusAnRcOnTheHeadCanBePromotedAndAnotherRcSaysNothingIsNew(t *testing.T) {
	ws := fixture(t)
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
	ws := fixture(t)
	ws.tags = []git.Tag{tag("v2.12.1")}
	ws.branches[1].Commit = "sha-v2.12.1"

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "released", v212.State)
	assert.Empty(t, v212.Next)
}

func TestStatusAStableTagWithWorkSinceItNamesTheNextPrerelease(t *testing.T) {
	ws := fixture(t)
	ws.tags = []git.Tag{tag("v2.12.1")}

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "work-since-release", v212.State)
	require.Equal(t, []string{"v2.12.2-rc.1"}, nextTags(v212))
	assert.True(t, v212.Next[0].Available)
}

func TestStatusALineWithNoTagStartsAtItsSeries(t *testing.T) {
	ws := fixture(t)
	ws.tags = nil

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Equal(t, "no-release", v212.State)
	assert.Equal(t, []string{"v2.12.0-rc.1"}, nextTags(v212))
}

func TestStatusABranchTheHostDoesNotHaveHasNoStateAndNothingNext(t *testing.T) {
	ws := fixture(t)
	ws.branches = ws.branches[:1]

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	v212 := byLine(got)["v2.12"]
	assert.Empty(t, v212.State)
	assert.Empty(t, v212.Next)
}

func TestRenderTextHasAReleasesSectionWithStateAndNextAndALocalCloneSection(t *testing.T) {
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)
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
	ws := fixture(t)
	ws.branches[0].Commit = "sha-v2.13.0-rc.10"
	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)
	require.NoError(t, err)

	out := renderText(got, nil)

	assert.Contains(t, out, "v2.13.0 (promotes v2.13.0-rc.10)")
	assert.Contains(t, out, "(v2.13.0-rc.11: no new commits)")
	assert.Contains(t, out, "rc open, head on the rc")
}

// shipping makes the fixture a project that takes from scc-operator, and
// scc-operator one that ships to it: both projects, the config that joins them,
// the refs of each and the file rancher holds the value in.
func shipping(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := fixture(t)
	ws.docs = append(rancherDocs(""),
		chaintest.Doc("Project", "scc-operator", "", "  repo: {name: rancher/scc-operator, owned: true}\n"),
		chaintest.Doc("Stream", "scc-operator", "{project: scc-operator}", "  universal: true\n"),
		chaintest.Doc("Artifact", "scc-image", "{stream: scc-operator}", "  kind: image\n  name: rancher/scc-operator\n"),
		chaintest.Doc("Holding", "rancher-holds-scc", "{project: rancher, artifact: scc-image}",
			"  at: {file: build.yaml, key: defaultSccOperatorImage}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"),
	)
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

// held is a consumer branch with what its one holding holds.
type held struct {
	Line, Branch, Current string
	Stable, Newest        *ship.Verdict
}

func holdingOn(t *testing.T, hs []ship.Holding, branch string) held {
	t.Helper()
	for _, h := range hs {
		if h.Branch == branch {
			require.Len(t, h.Held, 1)
			v := h.Held[0]
			return held{Line: h.Line, Branch: h.Branch, Current: v.Current, Stable: v.Stable, Newest: v.Newest}
		}
	}
	require.Failf(t, "no such holding", "%s in %v", branch, hs)
	return held{}
}

func TestStatusOfAConsumerHasASourceForEachProjectThatShipsToIt(t *testing.T) {
	ws := shipping(t)

	got, err := gather(ctx, ws, opener(syncedClone(), nil), []string{"rancher/rancher"})

	require.NoError(t, err)
	require.Len(t, got.Sources, 1)
	src := got.Sources[0]
	assert.Equal(t, "rancher/scc-operator", src.Producer)
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
	got, err := gather(ctx, fixture(t), opener(syncedClone(), nil), nil)

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
	ws := fixture(t)
	engine, err := bump.NewEngine(bump.Builtin(), chaintest.Projects(t))
	require.NoError(t, err)
	ws.engine, ws.engineErr = engine, errors.New("holding rancher-holds-scc is not usable")

	got, err := gather(ctx, ws, opener(syncedClone(), nil), nil)

	require.NoError(t, err)
	assert.Contains(t, got.TraceError, "not usable")
	assert.Len(t, got.Targets, 2, "the releases are reported all the same")
}

func TestHoldStatus(t *testing.T) {
	stable := func(tag string, upToDate bool, rel bump.Relation) *ship.Verdict {
		return &ship.Verdict{Tag: tag, UpToDate: upToDate, Relation: rel}
	}
	const (
		newer = bump.RelationNewer
		same  = bump.RelationSame
		older = bump.RelationOlder
	)
	tests := []struct {
		name string
		h    ship.HeldValue
		want string
	}{
		{"an error", ship.HeldValue{Error: "no value for the key"}, "unreadable: no value for the key"},
		{"nothing read", ship.HeldValue{}, "-"},
		{"on the release, no prerelease newer", ship.HeldValue{Stable: stable("v0.5.3", true, same), Newest: stable("v0.5.3", true, same)}, "current"},
		{"on the release, no newest at all", ship.HeldValue{Stable: stable("v0.5.3", true, same)}, "current"},
		{"behind the release", ship.HeldValue{Stable: stable("v0.5.3", false, newer), Newest: stable("v0.5.3", false, newer)}, "behind v0.5.3"},
		{"ahead of the release", ship.HeldValue{Stable: stable("v0.5.3", false, older)}, "ahead of v0.5.3"},
		{"a version it cannot compare", ship.HeldValue{Stable: stable("v0.5.3", false, bump.RelationUnknown)}, "unknown"},
		{"on the newest, a prerelease", ship.HeldValue{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.1", true, same)}, "current"},
		{"on the stable, a prerelease is newer", ship.HeldValue{Stable: stable("v0.5.3", true, same), Newest: stable("v0.6.0-rc.1", false, newer)}, "on stable, v0.6.0-rc.1 is newer"},
		{"behind the stable, a prerelease is newer", ship.HeldValue{Stable: stable("v0.5.3", false, newer), Newest: stable("v0.6.0-rc.1", false, newer)}, "behind v0.5.3"},
		{"between the stable and the newest", ship.HeldValue{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.2", false, newer)}, "on a prerelease, v0.6.0-rc.2 is newer"},
		{"past the newest", ship.HeldValue{Stable: stable("v0.5.3", false, older), Newest: stable("v0.6.0-rc.1", false, older)}, "ahead of v0.6.0-rc.1"},
		{"current, but the locations differ", ship.HeldValue{Stable: stable("v0.5.3", true, same), Differ: true}, "current; locations differ"},
		{"behind, and the locations differ", ship.HeldValue{Stable: stable("v0.5.3", false, newer), Differ: true}, "behind v0.5.3; locations differ"},
		{"nothing to compare, and the locations differ", ship.HeldValue{Differ: true}, "locations differ"},
		{"the locations differ and the document says why", ship.HeldValue{Stable: stable("v0.5.3", false, newer), Differ: true, DifferReason: "lag by design"}, "behind v0.5.3"},
		{"an error says nothing of the locations", ship.HeldValue{Error: "no value for the key", Differ: true}, "unreadable: no value for the key"},
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

	_, sources, found := strings.Cut(out, "\nSources: scc-operator (rancher/scc-operator)\n")
	require.True(t, found, out)
	for _, want := range []string{"LINE", "BRANCH", "HEAD", "HOLDING", "HOLDS", "LATEST STABLE", "STATUS",
		"rancher/scc-operator:v0.5.3", "v0.5.3", "on stable, v0.6.0-rc.1 is newer", "behind v0.5.3", "release/v2.12"} {
		assert.Contains(t, sources, want)
	}
	assert.NotContains(t, out, "Consumers:")
}

func TestRenderTextHasAConsumersSectionForWhatTheProjectShips(t *testing.T) {
	got, err := gather(ctx, shipping(t), opener(syncedClone(), nil), []string{"rancher/scc-operator"})
	require.NoError(t, err)

	out := renderText(got, nil)

	assert.Contains(t, out, "\nConsumers: rancher (rancher/rancher)\n")
	assert.NotContains(t, out, "Sources:")
}

func TestRenderTextAConsumerThatCouldNotBeTracedSaysWhy(t *testing.T) {
	st := Status{Slug: "rancher/scc-operator", Consumers: []ship.ConsumerHoldings{
		{Consumer: "rancher/rancher", Name: "rancher", Error: "read the branches of rancher/rancher: network down"},
	}, TraceError: "method x is not usable"}

	out := renderText(st, nil)

	assert.Contains(t, out, "Consumers: rancher (rancher/rancher)\n  unavailable: read the branches of rancher/rancher: network down")
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

func TestRenderTextShowsEachDistinctValueOfALocationThatDiffers(t *testing.T) {
	held := func(v ship.HeldValue) Status {
		return Status{Slug: "rancher/wrangler", Consumers: []ship.ConsumerHoldings{{
			Consumer: "rancher/rancher", Name: "rancher",
			Targets: []ship.Holding{{Line: "v2.16", Branch: "main", Held: []ship.HeldValue{v}}},
		}}}
	}
	differ := ship.HeldValue{
		Name: "rancher-holds-wrangler", Current: "v3.8.0-rc.3", Differ: true,
		Locations: []ship.HeldLocation{
			{File: "go.mod", Current: "v3.8.0-rc.3"}, {File: "pkg/apis/go.mod", Current: "v3.8.0-rc.3"}, {File: "pkg/client/go.mod", Current: "v3.8.0-rc.2"},
		},
		Stable: &ship.Verdict{Tag: "v3.8.0", UpToDate: false, Relation: bump.RelationNewer},
	}

	out := renderText(held(differ), nil)
	assert.Contains(t, out, "v3.8.0-rc.3, v3.8.0-rc.2", "each value once, in the order the locations list them")
	assert.Contains(t, out, "behind v3.8.0; locations differ")

	acknowledged := differ
	acknowledged.DifferReason = "nested modules lag by design"
	quiet := renderText(held(acknowledged), nil)
	assert.Contains(t, quiet, "v3.8.0-rc.3, v3.8.0-rc.2", "the values are still shown")
	assert.NotContains(t, quiet, "locations differ", "an acknowledged difference is not announced")
	assert.NotContains(t, quiet, "nested modules lag by design")

	agree := ship.HeldValue{Name: "h", Current: "v3.8.0", Locations: []ship.HeldLocation{{File: "a", Current: "v3.8.0"}, {File: "b", Current: "v3.8.0"}}}
	assert.NotContains(t, renderText(held(agree), nil), "v3.8.0, v3.8.0", "locations that agree show one value")
}

func TestRenderTextNamesTheStreamWhenAProjectIsTakenFromForSeveralOfThem(t *testing.T) {
	held := []ship.Holding{{Line: "v2.16", Branch: "main", Held: []ship.HeldValue{{Name: "h", Current: "v3.7.2"}}}}

	several := renderText(Status{Slug: "rancher/backup-restore-operator", Sources: []ship.SourceHoldings{
		{Producer: "rancher/wrangler", Name: "wrangler", Stream: "wrangler-v3-7", Targets: held},
		{Producer: "rancher/wrangler", Name: "wrangler", Stream: "wrangler-v3-3", Targets: held},
	}}, nil)
	assert.Contains(t, several, "Sources: wrangler (rancher/wrangler), stream wrangler-v3-7\n")
	assert.Contains(t, several, "Sources: wrangler (rancher/wrangler), stream wrangler-v3-3\n")

	one := renderText(Status{Slug: "rancher/backup-restore-operator", Sources: []ship.SourceHoldings{
		{Producer: "rancher/wrangler", Name: "wrangler", Stream: "wrangler-v3-7", Targets: held},
		{Producer: "rancher/lasso", Name: "lasso", Stream: "lasso-v0-2", Targets: held},
	}}, nil)
	assert.Contains(t, one, "Sources: wrangler (rancher/wrangler)\n", "one stream of each project needs no stream in the heading")
	assert.Contains(t, one, "Sources: lasso (rancher/lasso)\n")
	assert.NotContains(t, one, "stream ")
}

func TestRenderTextNamesTheStreamOfAConsumerThatTakesSeveralOfThem(t *testing.T) {
	held := []ship.Holding{{Line: "v2.16", Branch: "main", Held: []ship.HeldValue{{Name: "h", Current: "v3.7.2"}}}}

	out := renderText(Status{Slug: "rancher/wrangler", Consumers: []ship.ConsumerHoldings{
		{Consumer: "rancher/backup-restore-operator", Name: "backup-restore-operator", Stream: "wrangler-v3-7", Targets: held},
		{Consumer: "rancher/backup-restore-operator", Name: "backup-restore-operator", Stream: "wrangler-v3-3", Targets: held},
		{Consumer: "rancher/rancher", Name: "rancher", Stream: "wrangler-v3-8", Targets: held},
	}}, nil)

	assert.Contains(t, out, "Consumers: backup-restore-operator (rancher/backup-restore-operator), stream wrangler-v3-7\n")
	assert.Contains(t, out, "Consumers: backup-restore-operator (rancher/backup-restore-operator), stream wrangler-v3-3\n")
	assert.Contains(t, out, "Consumers: rancher (rancher/rancher)\n", "rancher takes one stream, so it needs none named")
}
