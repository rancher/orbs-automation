package doctor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorkspace answers the Workspace questions from fixed values and records
// the slugs it was asked to resolve.
type fakeWorkspace struct {
	dir      workspace.CentralConfigDir
	bundle   *bundlestore.Source // when set, the config is from the bundle
	dirErr   error
	graph    *chain.Graph
	chainErr error
	paths    []workspace.RepoPath
	asked    []string
}

func (f *fakeWorkspace) ConfigSource() (workspace.ConfigSource, error) {
	return workspace.ConfigSource{Dir: f.dir, Bundle: f.bundle}, f.dirErr
}
func (f *fakeWorkspace) Chain() (*chain.Graph, error) { return f.graph, f.chainErr }

func (f *fakeWorkspace) RepoPaths(slugs []string) []workspace.RepoPath {
	f.asked = slugs
	return f.paths
}

func projects(t *testing.T, slugs ...string) *chain.Graph {
	t.Helper()
	var pairs []string
	for i, slug := range slugs {
		pairs = append(pairs, fmt.Sprintf("p%d", i), slug)
	}
	return chaintest.Projects(t, pairs...)
}

func TestConfigDirCheckMetadata(t *testing.T) {
	c := NewConfigDirCheck(&fakeWorkspace{})

	assert.Equal(t, "central-config-dir", c.Name())
	assert.Equal(t, "configuration", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestConfigDirCheckFound(t *testing.T) {
	tests := []struct {
		name string
		dir  workspace.CentralConfigDir
		want string
	}{
		{"in the working directory", workspace.CentralConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}, "/work/config (cwd)"},
		{"from the environment", workspace.CentralConfigDir{Dir: "/etc/gero", Source: "env"}, "/etc/gero (env)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigDirCheck(&fakeWorkspace{dir: tt.dir}).Run(context.Background())

			assert.Equal(t, Result{Status: StatusOK, Message: tt.want}, got)
		})
	}
}

func TestConfigDirCheckMissingFailsWithTheAdviceAndTheBundle(t *testing.T) {
	got := NewConfigDirCheck(&fakeWorkspace{dirErr: workspace.ErrNotFound}).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusFail,
		Message:     "no central config directory found",
		Remediation: workspace.NotFoundHint + "; or run gero doctor --online to fetch the central config bundle",
	}, got)
}

func TestConfigDirCheckSaysWhenTheConfigIsTheBundle(t *testing.T) {
	ws := &fakeWorkspace{bundle: &bundlestore.Source{Tag: "v0.5.0", Age: 3 * time.Hour, FromCache: true}}

	got := NewConfigDirCheck(ws).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "bundle v0.5.0, 3 hours old"}, got)
}

func TestConfigDirCheckWarnsWhenTheBundleIsAStaleCacheAndSaysWhy(t *testing.T) {
	ws := &fakeWorkspace{bundle: &bundlestore.Source{
		Tag: "v0.3.0", Age: 72 * time.Hour, FromCache: true, Stale: true,
		Note: "using the cached bundle of v0.3.0, 3 days old: the GitHub rate limit is used up",
	}}

	got := NewConfigDirCheck(ws).Run(context.Background())

	assert.Equal(t, Result{
		Status: StatusWarn, Message: "bundle v0.3.0, 3 days old",
		Remediation: "using the cached bundle of v0.3.0, 3 days old: the GitHub rate limit is used up",
	}, got)
}

func TestConfigDirCheckWhenTheBundleFailedSaysWhyToo(t *testing.T) {
	err := fmt.Errorf("%w; the central config bundle is not available either: no config bundle is available: none is cached, and this run is offline", workspace.ErrNotFound)

	got := NewConfigDirCheck(&fakeWorkspace{dirErr: err}).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Equal(t, "no central config directory found", got.Message)
	assert.Contains(t, got.Remediation, "none is cached, and this run is offline")
	assert.Contains(t, got.Remediation, workspace.NotFoundHint)
}

func TestDoctorIsOfflineUnlessAskedToBeOnline(t *testing.T) {
	assert.True(t, forDoctor(clicfg.Settings{}, false).Offline, "the central bundle is read from the cache only")
	assert.False(t, forDoctor(clicfg.Settings{}, true).Offline, "--online may fetch it")
	assert.True(t, forDoctor(clicfg.Settings{Offline: true}, true).Offline, "GERO_OFFLINE still holds")
}

func TestConfigDirCheckOtherErrorFails(t *testing.T) {
	got := NewConfigDirCheck(&fakeWorkspace{dirErr: errors.New("permission denied")}).Run(context.Background())

	assert.Equal(t, Result{Status: StatusFail, Message: "permission denied"}, got)
}

func TestConfigCheckMetadata(t *testing.T) {
	c := NewConfigCheck(&fakeWorkspace{})

	assert.Equal(t, "config", c.Name())
	assert.Equal(t, "configuration", c.Category())
	assert.False(t, c.Online())
	assert.Equal(t, []string{"central-config-dir"}, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestConfigCheckValid(t *testing.T) {
	full := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "a", "", "  repo: {name: acme/a}\n"),
		chaintest.Doc("Project", "b", "", "  repo: {name: acme/b}\n"),
		chaintest.Doc("Stream", "s", "{project: a}", "  universal: true\n"),
		chaintest.Doc("Artifact", "img", "{stream: s}", "  kind: image\n  name: acme/a\n"),
		chaintest.Doc("Holding", "h", "{project: b, artifact: img}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release}\n"),
	)
	tests := []struct {
		name  string
		graph *chain.Graph
		want  string
	}{
		{"counts what it loaded", full, "2 projects, 1 stream, 1 artifact, 1 holding, 2 lines"},
		{"an empty config", chaintest.Projects(t), "0 projects, 0 streams, 0 artifacts, 0 holdings, 0 lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigCheck(&fakeWorkspace{graph: tt.graph}).Run(context.Background())

			assert.Equal(t, Result{Status: StatusOK, Message: tt.want}, got)
		})
	}
}

func TestConfigCheckWarnsOfTheConfigsWarnings(t *testing.T) {
	g, rep := chaintest.LoadFS(fstest.MapFS{
		"lines.yaml": {Data: []byte("apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n")},
		"p.yaml":     {Data: []byte(chaintest.Doc("Project", "p", "", "  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n"))},
	})
	require.NoError(t, rep.Err())

	got := NewConfigCheck(&fakeWorkspace{graph: g}).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusWarn,
		Message:     "1 project, 0 streams, 0 artifacts, 0 holdings, 1 line, 1 warning",
		Remediation: "run `gero config validate` for the details",
	}, got)
}

func TestConfigCheckDoesNotCountAnAcknowledgedWarning(t *testing.T) {
	g, rep := chaintest.LoadFS(fstest.MapFS{
		"lines.yaml": {Data: []byte("apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n")},
		"p.yaml":     {Data: []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: p\nacknowledge:\n  - {rule: default-branch-past-development, reason: read from main by design}\nspec:\n  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n")},
	})
	require.NoError(t, rep.Err())

	got := NewConfigCheck(&fakeWorkspace{graph: g}).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "1 project, 0 streams, 0 artifacts, 0 holdings, 1 line"}, got)
}

func TestConfigCheckProblemsFail(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"several problems", errors.Join(errors.New("a"), errors.New("b")), "2 problems in the central config"},
		{"one problem", errors.New("a"), "1 problem in the central config"},
		{
			// The shape the workspace returns: the join, wrapped with the directory.
			name: "wrapped with the config directory",
			err:  fmt.Errorf("config directory /work/config: %w", errors.Join(errors.New("a"), errors.New("b"), errors.New("c"))),
			want: "3 problems in the central config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigCheck(&fakeWorkspace{chainErr: tt.err}).Run(context.Background())

			assert.Equal(t, Result{
				Status:      StatusFail,
				Message:     tt.want,
				Remediation: "run `gero config validate` for the details",
			}, got)
		})
	}
}

func TestReposCheckMetadata(t *testing.T) {
	c := NewReposCheck(&fakeWorkspace{})

	assert.Equal(t, "repos", c.Name())
	assert.Equal(t, "configuration", c.Category())
	assert.False(t, c.Online())
	assert.Equal(t, []string{"config"}, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestReposCheckAllResolved(t *testing.T) {
	ws := &fakeWorkspace{
		graph: projects(t, "o/a", "o/b"),
		paths: []workspace.RepoPath{
			{Slug: "o/a", Path: "/src/a", Source: workspace.PathCwd},
			{Slug: "o/b", Path: "/src/b", Source: workspace.PathSibling},
		},
	}

	got := NewReposCheck(ws).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "2 of 2 projects have a local clone"}, got)
	assert.Equal(t, []string{"o/a", "o/b"}, ws.asked)
}

func TestReposCheckWithNoProjects(t *testing.T) {
	ws := &fakeWorkspace{graph: chaintest.Projects(t)}

	got := NewReposCheck(ws).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "no projects in the central config"}, got)
}

func TestReposCheckUnresolvedProjectsWarn(t *testing.T) {
	tests := []struct {
		name            string
		paths           []workspace.RepoPath
		wantMessage     string
		wantRemediation string
	}{
		{
			name: "not found",
			paths: []workspace.RepoPath{
				{Slug: "o/a", Path: "/src/a", Source: workspace.PathCwd},
				{Slug: "o/b", Err: workspace.ErrRepoNotFound},
				{Slug: "o/c", Path: "/src/c", Source: workspace.PathScan},
			},
			wantMessage: "1 of 3 projects has no unique local clone: o/b (not found)",
			wantRemediation: "not found: clone it under a repos.search directory, or set repos.paths.<slug>; " +
				"run `gero config resolve repos` for the details",
		},
		{
			name: "ambiguous",
			paths: []workspace.RepoPath{
				{Slug: "o/a", Err: workspace.ErrRepoAmbiguous, Candidates: []string{"/x/a", "/y/a"}},
				{Slug: "o/b", Path: "/src/b", Source: workspace.PathCwd},
			},
			wantMessage: "1 of 2 projects has no unique local clone: o/a (ambiguous)",
			wantRemediation: "ambiguous: set repos.paths.<slug> to choose one; " +
				"run `gero config resolve repos` for the details",
		},
		{
			name: "both kinds",
			paths: []workspace.RepoPath{
				{Slug: "o/a", Err: workspace.ErrRepoNotFound},
				{Slug: "o/b", Err: workspace.ErrRepoAmbiguous},
				{Slug: "o/c", Path: "/src/c", Source: workspace.PathCwd},
			},
			wantMessage: "2 of 3 projects have no unique local clone: o/a (not found), o/b (ambiguous)",
			wantRemediation: "not found: clone it under a repos.search directory, or set repos.paths.<slug>; " +
				"ambiguous: set repos.paths.<slug> to choose one; run `gero config resolve repos` for the details",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slugs := make([]string, len(tt.paths))
			for i, p := range tt.paths {
				slugs[i] = p.Slug
			}
			ws := &fakeWorkspace{graph: projects(t, slugs...), paths: tt.paths}

			got := NewReposCheck(ws).Run(context.Background())

			assert.Equal(t, Result{Status: StatusWarn, Message: tt.wantMessage, Remediation: tt.wantRemediation}, got)
		})
	}
}

func TestReposCheckThatCannotReadTheConfigWarns(t *testing.T) {
	got := NewReposCheck(&fakeWorkspace{chainErr: errors.New("boom")}).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusWarn,
		Message:     "could not read the central config: boom",
		Remediation: "run `gero config validate` for the details",
	}, got)
}

func TestReposCheckLeavesOutARepoOutsideTheOrganization(t *testing.T) {
	ws := &fakeWorkspace{
		graph: chaintest.Load(t,
			chaintest.Doc("Project", "mine", "", "  repo: {name: o/mine}\n"),
			chaintest.Doc("Project", "theirs", "", "  repo: {name: o/theirs, external: true}\n"),
		),
		paths: []workspace.RepoPath{{Slug: "o/mine", Path: "/src/mine", Source: workspace.PathCwd}},
	}

	got := NewReposCheck(ws).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "1 of 1 project has a local clone"}, got)
	assert.Equal(t, []string{"o/mine"}, ws.asked)
}
