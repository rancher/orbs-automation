package repoinfo

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

// fakeWorkspace answers Project from projects and from current, the Project of
// the working directory, and records what it was asked.
type fakeWorkspace struct {
	projects   map[string]config.Project
	current    string
	projectErr error
	paths      map[string]workspace.RepoPath
	tags       []git.Tag
	tagsErr    error
	asked      string
	tagCalls   []string
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

func (f *fakeWorkspace) UpstreamTags(_ context.Context, slug string) ([]git.Tag, error) {
	f.tagCalls = append(f.tagCalls, slug)
	return f.tags, f.tagsErr
}

type fakeClone struct {
	ref, short string
	headErr    error
}

func (f fakeClone) Head() (string, string, error) { return f.ref, f.short, f.headErr }

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
	}
}

var cloneOnMain = fakeClone{ref: "refs/heads/main", short: "abc1234"}

var ctx = context.Background()

func TestInfoConfigAndLocalState(t *testing.T) {
	ws := fixture()
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), []string{"rancher/rancher"}, Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", ws.asked)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.True(t, got.Owned)
	assert.Equal(t, "perLine", got.Release.Branches)
	assert.True(t, got.Release.RequiresPrerelease)
	assert.Equal(t, []LineInfo{
		{Line: "v2.13", Series: "v2.13", Branch: "main", IsDefaultBranch: true},
		{Line: "v2.12", Series: "v2.12", Branch: "release/v2.12"},
	}, got.Release.Lines, "a line with no branch uses the default branch, and a line's series defaults to the line")
	assert.Equal(t, LocalInfo{
		Path: "/src/rancher", Source: workspace.PathCwd, Branch: "main", Commit: "abc1234",
	}, got.Local)
}

func TestInfoOfflineReadsNoTags(t *testing.T) {
	ws := fixture()
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{Offline: true})
	require.NoError(t, err)
	assert.Nil(t, got.Upstream)
	assert.Empty(t, ws.tagCalls, "offline means no request")
}

func TestInfoUpstreamTags(t *testing.T) {
	ws := fixture()
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/rancher"}, ws.tagCalls)

	require.NotNil(t, got.Upstream)
	assert.Equal(t, info("v2.12.1"), got.Upstream.LatestStable)
	assert.Equal(t, info("v2.13.0-rc.10"), got.Upstream.LatestPrerelease, "rc.10 is after rc.2")
	assert.Empty(t, got.Upstream.Error)
}

func TestInfoUpstreamTagsPerLine(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)

	byLine := map[string]LineInfo{}
	for _, l := range got.Release.Lines {
		byLine[l.Line] = l
	}
	assert.Nil(t, byLine["v2.13"].LatestStable, "nothing stable in v2.13 yet")
	assert.Equal(t, info("v2.13.0-rc.10"), byLine["v2.13"].LatestPrerelease)
	assert.Equal(t, info("v2.12.1"), byLine["v2.12"].LatestStable)
	assert.Equal(t, info("v2.12.2-rc.1"), byLine["v2.12"].LatestPrerelease, "newer than v2.12.1, so a release is coming")
}

func TestInfoUpstreamTagsFollowTheTagPattern(t *testing.T) {
	ws := fixture()
	p := ws.projects["rancher/rancher"]
	p.Release.TagPattern = "other-*"
	ws.projects["rancher/rancher"] = p
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	assert.Nil(t, got.Upstream.LatestStable)
	assert.Nil(t, got.Upstream.LatestPrerelease)
}

func TestInfoUpstreamErrorKeepsTheRest(t *testing.T) {
	ws := fixture()
	ws.tagsErr = errors.New("network down")
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	require.NotNil(t, got.Upstream)
	assert.Equal(t, "network down", got.Upstream.Error)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Equal(t, "abc1234", got.Local.Commit)
	assert.Nil(t, got.Release.Lines[0].LatestStable)
}

func TestInfoFromCurrentRepository(t *testing.T) {
	ws := fixture()
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{Offline: true})
	require.NoError(t, err)
	assert.Empty(t, ws.asked, "an empty slug asks for the working directory's Project")
	assert.Equal(t, "rancher/rancher", got.Slug)
}

func TestInfoProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		ws := fixture()
		ws.projectErr = want
		_, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
		assert.ErrorIs(t, err, want)
	}
}

func TestInfoUnresolvedCloneKeepsTheRest(t *testing.T) {
	ws := fixture()
	ws.paths["rancher/rancher"] = workspace.RepoPath{Err: workspace.ErrRepoNotFound}
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), []string{"rancher/rancher"}, Options{})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Equal(t, LocalInfo{Error: workspace.ErrRepoNotFound.Error()}, got.Local)
	require.NotNil(t, got.Upstream, "the upstream does not need a clone")
	assert.Equal(t, info("v2.12.1"), got.Upstream.LatestStable)
}

func TestInfoDetachedHead(t *testing.T) {
	c := cloneOnMain
	c.ref, c.headErr = "", workspace.ErrDetachedHead
	got, err := gather(ctx, fixture(), opener(c, nil), nil, Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, "(detached)", got.Local.Branch)
}

func TestInfoUnreadableClone(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(nil, errors.New("boom")), nil, Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, "/src/rancher", got.Local.Path)
	assert.Equal(t, "boom", got.Local.Error)
	assert.Empty(t, got.Local.Branch)
}

func TestRenderTextHeadings(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	out := renderText(got)

	for _, want := range []string{
		"rancher/rancher\n", "\nRelease lines\n", "\nLatest releases (upstream)\n", "\nLocal clone\n",
	} {
		assert.Contains(t, out, want)
	}
	for _, want := range []string{
		"owned", "yes", "default branch", "main", "perLine",
		"LATEST STABLE", "LATEST PRE-RELEASE",
		"v2.13", "main (default branch)", "release/v2.12",
		"v2.12.1 (sha-v2.", // the commit is shown short; the fake's is shorter than 7 characters
		"/src/rancher", "cwd", "abc1234",
	} {
		assert.Contains(t, out, want)
	}
	assert.Less(t, indexOf(out, "Release lines"), indexOf(out, "Latest releases (upstream)"))
	assert.Less(t, indexOf(out, "Latest releases (upstream)"), indexOf(out, "Local clone"))
}

func indexOf(s, sub string) int {
	for i := range s {
		if len(s[i:]) >= len(sub) && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestRenderTextOfflineSaysTagsWereNotRead(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(cloneOnMain, nil), nil, Options{Offline: true})
	require.NoError(t, err)
	out := renderText(got)
	assert.Contains(t, out, "not read (--offline)")
	assert.NotContains(t, out, "LATEST STABLE", "the table has no tag columns without tags")
}

func TestRenderTextUpstreamError(t *testing.T) {
	ws := fixture()
	ws.tagsErr = errors.New("network down")
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	out := renderText(got)
	assert.Contains(t, out, "unavailable: network down")
	assert.NotContains(t, out, "LATEST STABLE")
}

func TestRenderTextShortensCommits(t *testing.T) {
	assert.Equal(t, "v1.0.0 (0123456)", tagText(&TagInfo{Tag: "v1.0.0", Commit: "0123456789abcdef"}))
	assert.Equal(t, "-", tagText(nil))
}

func TestRenderTextShowsLocalError(t *testing.T) {
	got := Info{Slug: "rancher/charts", Local: LocalInfo{Error: "no local clone found"}}
	assert.Contains(t, renderText(got), "no local clone found")
}

func TestRenderTextSingleProjectHasNoLinesTable(t *testing.T) {
	ws := fixture()
	p := ws.projects["rancher/rancher"]
	p.Release.Branches, p.Release.Lines = "single", nil
	ws.projects["rancher/rancher"] = p
	got, err := gather(ctx, ws, opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	out := renderText(got)
	assert.NotContains(t, out, "Release lines")
	assert.Contains(t, out, "Latest releases (upstream)")
}

func TestRenderJSON(t *testing.T) {
	got, err := gather(ctx, fixture(), opener(cloneOnMain, nil), nil, Options{})
	require.NoError(t, err)
	out, err := renderJSON(got)
	require.NoError(t, err)
	var back Info
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, got, back)
	assert.Contains(t, out, `"commit": "sha-v2.12.1"`, "JSON carries the full commit")
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	s := clicfg.Settings{Output: clicfg.OutputEnv}
	_, err := Run(ctx, s, t.TempDir(), nil, Options{})
	assert.ErrorContains(t, err, "not supported by repo info")
}
