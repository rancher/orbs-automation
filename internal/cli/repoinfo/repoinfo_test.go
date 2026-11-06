package repoinfo

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
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
	asked      string
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

func fixture() *fakeWorkspace {
	return &fakeWorkspace{
		projects: map[string]config.Project{"rancher/rancher": rancherProject()},
		current:  "rancher/rancher",
		paths:    map[string]workspace.RepoPath{"rancher/rancher": {Path: "/src/rancher", Source: workspace.PathCwd}},
	}
}

var cloneOnMain = fakeClone{ref: "refs/heads/main", short: "abc1234"}

func TestInfoConfigAndLocalState(t *testing.T) {
	ws := fixture()
	got, err := gather(ws, opener(cloneOnMain, nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", ws.asked)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.True(t, got.Owned)
	assert.Equal(t, "perLine", got.Release.Branching)
	assert.True(t, got.Release.RequiresPrerelease)
	assert.Equal(t, []LineInfo{
		{Line: "v2.13", Series: "v2.13", Branch: "main", IsDefaultBranch: true},
		{Line: "v2.12", Series: "v2.12", Branch: "release/v2.12"},
	}, got.Release.Lines, "a line with no branch uses the default branch, and a line's series defaults to the line")
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
	ws := fixture()
	got, err := gather(ws, opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.Empty(t, ws.asked, "an empty slug asks for the working directory's Project")
	assert.Equal(t, "rancher/rancher", got.Slug)
}

func TestInfoProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		ws := fixture()
		ws.projectErr = want
		_, err := gather(ws, opener(cloneOnMain, nil), nil)
		assert.ErrorIs(t, err, want)
	}
}

func TestInfoUnresolvedCloneKeepsTheRest(t *testing.T) {
	ws := fixture()
	ws.paths["rancher/rancher"] = workspace.RepoPath{Err: workspace.ErrRepoNotFound}
	got, err := gather(ws, opener(cloneOnMain, nil), []string{"rancher/rancher"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", got.Slug)
	assert.Equal(t, LocalInfo{Error: workspace.ErrRepoNotFound.Error()}, got.Local)
	assert.Contains(t, renderText(got), "unavailable")
}

func TestInfoDetachedHead(t *testing.T) {
	c := cloneOnMain
	c.ref, c.headErr = "", workspace.ErrDetachedHead
	got, err := gather(fixture(), opener(c, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "(detached)", got.Local.Branch)
	assert.False(t, got.Local.Dirty, "a detached HEAD alone is not a change")
}

func TestInfoDirtyClone(t *testing.T) {
	c := cloneOnMain
	c.dirty = true
	got, err := gather(fixture(), opener(c, nil), nil)
	require.NoError(t, err)
	assert.True(t, got.Local.Dirty)
	assert.Contains(t, renderText(got), "changes or an operation in progress")

	got, err = gather(fixture(), opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.False(t, got.Local.Dirty)
	assert.Contains(t, renderText(got), "clean")
}

func TestInfoUnreadableClone(t *testing.T) {
	got, err := gather(fixture(), opener(nil, errors.New("boom")), nil)
	require.NoError(t, err)
	assert.Equal(t, "/src/rancher", got.Local.Path)
	assert.Equal(t, "boom", got.Local.Error)
	assert.Empty(t, got.Local.Branch)
}

func TestInfoUnreadableWorkTree(t *testing.T) {
	c := cloneOnMain
	c.cleanErr = errors.New("status failed")
	got, err := gather(fixture(), opener(c, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "status failed", got.Local.Error)
	assert.False(t, got.Local.Dirty)
}

func TestRenderTextHeadings(t *testing.T) {
	got, err := gather(fixture(), opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	out := renderText(got)

	for _, want := range []string{"rancher/rancher\n", "\nRelease lines\n", "\nLocal clone\n"} {
		assert.Contains(t, out, want)
	}
	for _, want := range []string{
		"owned", "yes", "default branch", "main", "perLine", "LINE", "SERIES", "BRANCH",
		"v2.13", "main (default branch)", "release/v2.12", "/src/rancher", "cwd", "abc1234",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "Upstream", "the host is not read")
	assert.Less(t, strings.Index(out, "Release lines"), strings.Index(out, "Local clone"))
}

func TestRenderTextShowsLocalError(t *testing.T) {
	got := Info{Slug: "rancher/charts", Local: LocalInfo{Error: "no local clone found"}}
	assert.Contains(t, renderText(got), "no local clone found")
}

func TestRenderTextSingleProjectHasNoLinesTable(t *testing.T) {
	ws := fixture()
	p := ws.projects["rancher/rancher"]
	p.Release.Branching, p.Release.Lines = "single", nil
	ws.projects["rancher/rancher"] = p
	got, err := gather(ws, opener(cloneOnMain, nil), nil)
	require.NoError(t, err)
	assert.NotContains(t, renderText(got), "Release lines")
}

func TestRenderJSON(t *testing.T) {
	got, err := gather(fixture(), opener(cloneOnMain, nil), nil)
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
