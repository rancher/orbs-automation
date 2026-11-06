package loader

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func project(name, body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("apiVersion: gero/v1alpha1\nkind: Project\nname: " + name + "\nspec:\n" + body)}
}

func TestLoadDecodesProject(t *testing.T) {
	fsys := fstest.MapFS{"rancher-charts.project.yaml": project("rancher-charts", `  repo:
    name: rancher/charts
    owned: true
  release:
    branching: perLine
    requiresPrerelease: true
    lines:
      - line: v2.16
        version: "12.x"
      - line: v2.15
        branch: dev-v2.15
  pullRequests:
    title: "[{branch}] bump"
    labels:
      - status/auto-created
    botLabels:
      - origin/bot
    pushTo: upstream
  updateMethods:
    - name: package-bump
      strategy: chartsPackageBump
      accept:
        when: all
      postEdit:
        - run: [go, generate, ./...]
`)}

	got, err := Load(fsys)

	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	assert.Equal(t, config.Project{
		Name: "rancher-charts",
		Repo: config.Repo{Name: "rancher/charts", Owned: true},
		Release: config.Release{
			Branching:          "perLine",
			RequiresPrerelease: true,
			Lines: []config.ProjectLine{
				{Line: "v2.16", Version: "12.x"},
				{Line: "v2.15", Branch: "dev-v2.15"},
			},
		},
		PullRequests: config.PullRequests{
			Title: "[{branch}] bump", Labels: []string{"status/auto-created"}, BotLabels: []string{"origin/bot"}, PushTo: "upstream",
		},
		UpdateMethods: []config.UpdateMethod{{
			Name:     "package-bump",
			Strategy: "chartsPackageBump",
			Accept:   config.Accept{When: "all"},
			PostEdit: []config.Step{{Run: config.Command{"go", "generate", "./..."}}},
		}},
	}, got.Projects[0])
}

func TestLoadDecodesRancherLines(t *testing.T) {
	fsys := fstest.MapFS{"rancher.lines.yaml": {Data: []byte(`apiVersion: gero/v1alpha1
kind: RancherLines
name: rancher
spec:
  lines:
    - id: v2.16
      phase: development
      kubernetes:
        min: "1.35"
        max: "1.37"
`)}}

	got, err := Load(fsys)

	require.NoError(t, err)
	require.NotNil(t, got.Lines)
	assert.Equal(t, []config.Line{{ID: "v2.16", Phase: "development", Kubernetes: config.KubernetesRange{Min: "1.35", Max: "1.37"}}}, got.Lines.Lines)
}

func TestLoadDecodesLineDates(t *testing.T) {
	fsys := fstest.MapFS{"rancher.lines.yaml": {Data: []byte(`apiVersion: gero/v1alpha1
kind: RancherLines
name: rancher
spec:
  lines:
    - id: v2.15
      dates:
        ga: 2026-08-27
        eom: 2027-02-27
        eol: 2028-02-27
      kubernetes:
        min: "1.34"
        max: "1.36"
`)}}

	got, err := Load(fsys)

	require.NoError(t, err)
	require.NotNil(t, got.Lines)
	assert.Equal(t, []config.Line{{
		ID: "v2.15",
		Dates: config.Dates{
			GA:  time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC),
			EOM: time.Date(2027, time.February, 27, 0, 0, 0, 0, time.UTC),
			EOL: time.Date(2028, time.February, 27, 0, 0, 0, 0, time.UTC),
		},
		Kubernetes: config.KubernetesRange{Min: "1.34", Max: "1.36"},
	}}, got.Lines.Lines)
}

func TestLoadDecodesExternal(t *testing.T) {
	fsys := fstest.MapFS{"example-sdk.external.yaml": {Data: []byte(`apiVersion: gero/v1alpha1
kind: External
name: example-sdk
spec:
  repo:
    name: third-party/example-sdk
  relationships:
    - project: backup-restore-operator
      action: trackOnly
`)}}

	got, err := Load(fsys)

	require.NoError(t, err)
	assert.Equal(t, []config.External{{
		Name:          "example-sdk",
		Repo:          config.Repo{Name: "third-party/example-sdk"},
		Relationships: []config.Relationship{{Project: "backup-restore-operator", Action: "trackOnly"}},
	}}, got.Externals)
}

func TestLoadMultiDocumentFile(t *testing.T) {
	fsys := fstest.MapFS{"two.project.yaml": {Data: []byte(`apiVersion: gero/v1alpha1
kind: Project
name: one
spec:
  repo:
    name: o/one
---
apiVersion: gero/v1alpha1
kind: Project
name: two
spec:
  repo:
    name: o/two
`)}}

	got, err := Load(fsys)

	require.NoError(t, err)
	require.Len(t, got.Projects, 2)
	assert.Equal(t, "two", got.Projects[1].Name)
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{
			"unknown spec key",
			fstest.MapFS{"a.project.yaml": project("a", "  repo:\n    name: o/a\n    nope: 1\n")},
			"a.project.yaml",
		},
		{
			"unsupported apiVersion",
			fstest.MapFS{"a.project.yaml": {Data: []byte("apiVersion: gero/v9\nkind: Project\nname: a\nspec: {}\n")}},
			"apiVersion",
		},
		{
			"kind does not match file suffix",
			fstest.MapFS{"a.external.yaml": project("a", "  repo:\n    name: o/a\n")},
			"kind",
		},
		{
			"unknown kind",
			fstest.MapFS{"a.project.yaml": {Data: []byte("apiVersion: gero/v1alpha1\nkind: Nope\nname: a\nspec: {}\n")}},
			"Nope",
		},
		{
			"duplicate project name across files",
			fstest.MapFS{
				"a.project.yaml": project("a", "  repo:\n    name: o/a\n"),
				"b.project.yaml": project("a", "  repo:\n    name: o/b\n"),
			},
			`duplicate`,
		},
		{
			"malformed yaml",
			fstest.MapFS{"a.project.yaml": {Data: []byte("kind: [unclosed\n")}},
			"a.project.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.fsys)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestLoadIgnoresUnrelatedFiles(t *testing.T) {
	fsys := fstest.MapFS{"README.md": {Data: []byte("# hi")}}

	got, err := Load(fsys)

	require.NoError(t, err)
	assert.Empty(t, got.Projects)
	assert.Nil(t, got.Lines)
}

// The checked-in directories are the contract the shapes were designed against:
// every file in them loads and validates. The mock directory is a fixed set of
// fixtures, so its count is pinned; the real one grows as projects are onboarded,
// so it is only required to hold the projects the rest of the tests lean on.
func TestLoadRepoConfigDirs(t *testing.T) {
	for dir, wantProjects := range map[string]int{"../../../config": 0, "../../../config-mock": 7} {
		t.Run(dir, func(t *testing.T) {
			got, err := Load(os.DirFS(dir))

			require.NoError(t, err)
			if wantProjects > 0 {
				assert.Len(t, got.Projects, wantProjects)
			} else {
				assert.NotEmpty(t, got.Projects)
			}
			require.NotNil(t, got.Lines)
			assert.Equal(t, "v2.16", got.Lines.Lines[0].ID)
		})
	}
}
