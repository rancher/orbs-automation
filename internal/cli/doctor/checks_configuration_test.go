package doctor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
)

// fakeWorkspace answers the Workspace questions from fixed values and records
// the slugs it was asked to resolve.
type fakeWorkspace struct {
	dir    workspace.ConfigDir
	dirErr error
	cfg    config.Config
	cfgErr error
	paths  []workspace.RepoPath
	asked  []string
}

func (f *fakeWorkspace) ConfigDir() (workspace.ConfigDir, error) { return f.dir, f.dirErr }
func (f *fakeWorkspace) Config() (config.Config, error)          { return f.cfg, f.cfgErr }

func (f *fakeWorkspace) RepoPaths(slugs []string) []workspace.RepoPath {
	f.asked = slugs
	return f.paths
}

func projects(slugs ...string) config.Config {
	var cfg config.Config
	for _, s := range slugs {
		cfg.Projects = append(cfg.Projects, config.Project{Repo: config.Repo{Name: s}})
	}
	return cfg
}

func TestConfigDirCheckMetadata(t *testing.T) {
	c := NewConfigDirCheck(&fakeWorkspace{})

	assert.Equal(t, "config-dir", c.Name())
	assert.Equal(t, "configuration", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestConfigDirCheckFound(t *testing.T) {
	tests := []struct {
		name string
		dir  workspace.ConfigDir
		want string
	}{
		{"in the working directory", workspace.ConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}, "/work/config (cwd)"},
		{"from the environment", workspace.ConfigDir{Dir: "/etc/gero", Source: "env"}, "/etc/gero (env)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigDirCheck(&fakeWorkspace{dir: tt.dir}).Run(context.Background())

			assert.Equal(t, Result{Status: StatusOK, Message: tt.want}, got)
		})
	}
}

func TestConfigDirCheckMissingFailsWithTheAdvice(t *testing.T) {
	got := NewConfigDirCheck(&fakeWorkspace{dirErr: workspace.ErrNotFound}).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusFail,
		Message:     "no central config directory found",
		Remediation: workspace.NotFoundHint,
	}, got)
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
	assert.Equal(t, []string{"config-dir"}, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestConfigCheckValid(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "counts what it loaded",
			cfg: config.Config{
				Projects:  make([]config.Project, 2),
				Externals: make([]config.External, 1),
				Lines:     &config.RancherLines{Lines: make([]config.Line, 5)},
			},
			want: "2 projects, 1 external, 5 lines",
		},
		{name: "an empty config", cfg: config.Config{}, want: "0 projects, 0 externals, 0 lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigCheck(&fakeWorkspace{cfg: tt.cfg}).Run(context.Background())

			assert.Equal(t, Result{Status: StatusOK, Message: tt.want}, got)
		})
	}
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
			got := NewConfigCheck(&fakeWorkspace{cfgErr: tt.err}).Run(context.Background())

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
		cfg: projects("o/a", "o/b"),
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
	ws := &fakeWorkspace{}

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
			ws := &fakeWorkspace{cfg: projects(slugs...), paths: tt.paths}

			got := NewReposCheck(ws).Run(context.Background())

			assert.Equal(t, Result{Status: StatusWarn, Message: tt.wantMessage, Remediation: tt.wantRemediation}, got)
		})
	}
}

func TestReposCheckThatCannotReadTheConfigWarns(t *testing.T) {
	got := NewReposCheck(&fakeWorkspace{cfgErr: errors.New("boom")}).Run(context.Background())

	assert.Equal(t, Result{
		Status:      StatusWarn,
		Message:     "could not read the central config: boom",
		Remediation: "run `gero config validate` for the details",
	}, got)
}
