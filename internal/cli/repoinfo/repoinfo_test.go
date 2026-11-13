package repoinfo

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorkspace answers Project from graph and from current, the slug of the
// Project of the working directory, and records what it was asked.
type fakeWorkspace struct {
	graph      *chain.Graph
	current    string
	projectErr error
	paths      map[string]workspace.RepoPath
	asked      string
}

func (f *fakeWorkspace) Project(key string) (*chain.Project, error) {
	f.asked = key
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	if key == "" {
		key = f.current
	}
	return f.graph.Project(key), nil
}

func (f *fakeWorkspace) RepoPath(slug string) (workspace.RepoPath, error) {
	p := f.paths[slug]
	p.Slug = slug
	return p, p.Err
}

type fakeClone struct {
	ref, short string
	headErr    error
	dirty      bool
	cleanErr   error
}

func (f fakeClone) Head() (string, string, error) { return f.ref, f.short, f.headErr }

func (f fakeClone) Clean() (bool, error) { return !f.dirty, f.cleanErr }

func opener(c Clone, err error) Opener {
	return func(string) (Clone, error) { return c, err }
}

// rancherGraph is a project with a development line that has no branch of its own,
// a line on a release branch, and one release stream with a series for each.
func rancherGraph(t *testing.T, repo string) *chain.Graph {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Lines("v2.13", "v2.12"),
		chaintest.Doc("Project", "rancher", "", "  repo: "+repo+"\n  branches:\n    layout: perLine\n    lines:\n      - {line: v2.13}\n      - {line: v2.12, branch: release/v2.12}\n"),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  requiresPrerelease: true\n  lines:\n    - {line: v2.13, series: \"2.13.x\"}\n    - {line: v2.12, series: \"2.12.x\"}\n"),
	)
}

func fixture(t *testing.T) *fakeWorkspace {
	t.Helper()
	return &fakeWorkspace{
		graph:   rancherGraph(t, "{name: rancher/rancher, managedBy: orbs}"),
		current: "rancher/rancher",
		paths:   map[string]workspace.RepoPath{"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathCwd}},
	}
}

var cloneOnMain = fakeClone{ref: "refs/heads/main", short: "abc1234"}

func TestInfoConfigAndLocalState(t *testing.T) {
	ws := fixture(t)
	got, err := gather(ws, "orbs", opener(cloneOnMain, nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", ws.asked)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.True(t, got.Owned)
	assert.Equal(t, "perLine", got.Branches.Layout)
	assert.Equal(t, []LineInfo{
		{Line: "v2.13", Branch: "main", IsDefaultBranch: true},
		{Line: "v2.12", Branch: "release/v2.12"},
	}, got.Branches.Lines, "a line with no branch uses the default branch")
	assert.Equal(t, []StreamInfo{{
		Name: "rancher", Scheme: "semver", TagPattern: "v*", RequiresPrerelease: true,
		Lines: []StreamLineInfo{{Line: "v2.13", Series: "2.13.x"}, {Line: "v2.12", Series: "2.12.x"}},
	}}, got.Streams)
	assert.Equal(t, LocalInfo{
		Path: "/src/rancher", Source: workspace.PathCwd, Branch: "main", Commit: "abc1234",
	}, got.Local)
}

// The Workspace and Clone it is given have no way to reach the network: this
// is the command's contract, kept by what the interfaces offer.
func TestInfoNeedsNothingFromTheHost(t *testing.T) {
	var _ Workspace = (*fakeWorkspace)(nil)
	assert.Equal(t, 2, reflect.TypeFor[Workspace]().NumMethod(), "Project and RepoPath only")
}

func TestInfoFromCurrentRepository(t *testing.T) {
	ws := fixture(t)
	got, err := gather(ws, "orbs", opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.Empty(t, ws.asked, "an empty slug asks for the working directory's Project")
	assert.Equal(t, "rancher/rancher", got.Slug)
}

func TestInfoProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		ws := fixture(t)
		ws.projectErr = want
		_, err := gather(ws, "orbs", opener(cloneOnMain, nil), nil)
		assert.ErrorIs(t, err, want)
	}
}

func TestInfoUnresolvedCloneKeepsTheRest(t *testing.T) {
	ws := fixture(t)
	ws.paths["rancher/rancher"] = workspace.RepoPath{Err: workspace.ErrRepoNotFound}
	got, err := gather(ws, "orbs", opener(cloneOnMain, nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Equal(t, LocalInfo{Error: workspace.ErrRepoNotFound.Error()}, got.Local)
	assert.Contains(t, renderText(got, nil), "unavailable")
}

func TestInfoDetachedHead(t *testing.T) {
	c := cloneOnMain
	c.ref, c.headErr = "", workspace.ErrDetachedHead
	got, err := gather(fixture(t), "orbs", opener(c, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "(detached)", got.Local.Branch)
	assert.False(t, got.Local.Dirty, "a detached HEAD alone is not a change")
}

func TestInfoDirtyClone(t *testing.T) {
	c := cloneOnMain
	c.dirty = true
	got, err := gather(fixture(t), "orbs", opener(c, nil), nil)
	require.NoError(t, err)
	assert.True(t, got.Local.Dirty)
	assert.Contains(t, renderText(got, nil), "changes or an operation in progress")

	got, err = gather(fixture(t), "orbs", opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.False(t, got.Local.Dirty)
	assert.Contains(t, renderText(got, nil), "clean")
}

func TestInfoUnreadableClone(t *testing.T) {
	got, err := gather(fixture(t), "orbs", opener(nil, errors.New("boom")), nil)
	require.NoError(t, err)
	assert.Equal(t, "/src/rancher", got.Local.Path)
	assert.Equal(t, "boom", got.Local.Error)
	assert.Empty(t, got.Local.Branch)
}

func TestInfoUnreadableWorkTree(t *testing.T) {
	c := cloneOnMain
	c.cleanErr = errors.New("status failed")
	got, err := gather(fixture(t), "orbs", opener(c, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "status failed", got.Local.Error)
	assert.False(t, got.Local.Dirty)
}

func TestRenderTextHeadings(t *testing.T) {
	got, err := gather(fixture(t), "orbs", opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	out := renderText(got, nil)

	for _, want := range []string{"rancher/rancher\n", "\nBranches\n", "\nStream rancher\n", "\nLocal clone\n"} {
		assert.Contains(t, out, want)
	}
	for _, want := range []string{
		"owned", "yes", "default branch", "main", "perLine", "LINE", "SERIES", "BRANCH",
		"v2.13", "main (default branch)", "release/v2.12", "2.13.x", "/src/rancher", "cwd", "abc1234",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "Upstream", "the host is not read")
	assert.Less(t, strings.Index(out, "Branches"), strings.Index(out, "Stream rancher"))
	assert.Less(t, strings.Index(out, "Stream rancher"), strings.Index(out, "Local clone"))
}

func TestRenderTextShowsLocalError(t *testing.T) {
	got := Info{Slug: "rancher/charts", Local: LocalInfo{Error: "no local clone found"}}
	assert.Contains(t, renderText(got, nil), "no local clone found")
}

func TestRenderTextSingleProjectHasNoLinesTable(t *testing.T) {
	ws := fixture(t)
	ws.graph = chaintest.Projects(t, "rancher", "rancher/rancher")
	got, err := gather(ws, "orbs", opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.NotContains(t, renderText(got, nil), "Branches\n")
	assert.Equal(t, "single", got.Branches.Layout)
}

func TestRenderJSON(t *testing.T) {
	got, err := gather(fixture(t), "orbs", opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	out, err := renderJSON(got)
	require.NoError(t, err)
	var back Info
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, got, back)
	assert.NotContains(t, out, "upstream")
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	s := clicfg.Settings{Output: clicfg.OutputEnv}
	_, err := Run(s, t.TempDir(), nil)
	assert.ErrorContains(t, err, "not supported by repo info")
}

func TestInfoSaysWhoManagesTheProjectAndWhetherTheActiveTeamOwnsIt(t *testing.T) {
	tests := []struct {
		name       string
		repo       string
		want       string
		wantOwned  bool
		wantShared bool
	}{
		{"the active team's", "{name: rancher/rancher, managedBy: orbs}", "orbs", true, false},
		{"another team's", "{name: rancher/rancher, managedBy: release-team}", "release-team", false, false},
		{"shared", "{name: rancher/rancher, managedBy: shared}", "shared", false, true},
		{"unset is unspecified", "{name: rancher/rancher}", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := fixture(t)
			ws.graph = rancherGraph(t, tt.repo)

			got, err := gather(ws, "orbs", opener(cloneOnMain, nil), nil)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.ManagedBy)
			assert.Equal(t, tt.wantOwned, got.Owned)
			assert.Equal(t, tt.wantShared, got.Shared)
		})
	}
}

func TestInfoOwnershipFollowsTheActiveTeam(t *testing.T) {
	ws := fixture(t)
	ws.graph = rancherGraph(t, "{name: rancher/rancher, managedBy: release-team}")

	got, err := gather(ws, "release-team", opener(cloneOnMain, nil), nil)

	require.NoError(t, err)
	assert.True(t, got.Owned, "the same project is owned by the team that manages it")
}

func TestRenderTextShowsManagedByNextToOwned(t *testing.T) {
	out := renderText(Info{Slug: "rancher/rancher", Owned: true, ManagedBy: "orbs"}, nil)
	assert.Regexp(t, `owned +yes\n +managed by +orbs\n`, out)

	out = renderText(Info{Slug: "rancher/rancher-assets"}, nil)
	assert.Regexp(t, `owned +no\n +managed by +unspecified\n`, out)
}

func TestInfoJSONCarriesManagedByAndLeavesItOutWhenUnspecified(t *testing.T) {
	with, err := renderJSON(Info{Slug: "a/b", ManagedBy: "release-team"})
	require.NoError(t, err)
	assert.Contains(t, with, `"managedBy": "release-team"`)

	without, err := renderJSON(Info{Slug: "a/b"})
	require.NoError(t, err)
	assert.NotContains(t, without, "managedBy")
}

func TestInfoShowsALineWithABranchForEachStageAndOneWithNoneYet(t *testing.T) {
	ws := fixture(t)
	ws.graph = chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "charts", "", "  repo: {name: rancher/charts}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines:\n      - {line: v2.16, branch: {dev: dev-v2.16, stable: release-v2.16}}\n      - {line: v2.15, branch: \"\"}\n"),
	)
	ws.current = "rancher/charts"

	got, err := gather(ws, "orbs", opener(cloneOnMain, nil), nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"dev", "stable"}, got.Branches.Stages)
	assert.Equal(t, []LineInfo{
		{Line: "v2.16", Stages: map[string]string{"dev": "dev-v2.16", "stable": "release-v2.16"}},
		{Line: "v2.15", Pending: true},
	}, got.Branches.Lines)
	out := renderText(got, nil)
	assert.Contains(t, out, "dev: dev-v2.16, stable: release-v2.16")
	assert.Contains(t, out, "(none yet)")
}
