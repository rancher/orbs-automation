package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

func TestTheGraphFindsAProjectByNameOrSlugWhateverTheCase(t *testing.T) {
	g := chaintest.Load(t,
		project("shell", "  repo: {name: rancher/shell}\n"),
		project("rancher", "  repo: {name: rancher/rancher}\n"),
	)

	require.Len(t, g.Projects(), 2)
	assert.Equal(t, "shell", g.Projects()[0].Name())
	assert.Equal(t, "rancher", g.Projects()[1].Name())
	assert.Equal(t, "rancher/shell", g.Project("shell").Slug())
	assert.Equal(t, "shell", g.Project("SHELL").Name())
	assert.Equal(t, "rancher", g.Project("Rancher/Rancher").Name())
	assert.Nil(t, g.Project("nope"))
	assert.Nil(t, g.Project("rancher/nope"))
	assert.Nil(t, g.Project(""))
}

func TestTheGraphHasTheLines(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15"))

	require.Len(t, g.Lines(), 2)
	assert.Equal(t, "v2.16", g.Lines()[0].ID)
	assert.Equal(t, "v2.15", g.Lines()[1].ID)
	assert.Empty(t, chaintest.Load(t, project("p", "  repo: {name: a/b}\n")).Lines())
}

func TestAProjectKnowsItsOwnFacts(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		project("b", "  repo: {name: acme/b, defaultBranch: master}\n"),
	)

	assert.Equal(t, "main", g.Project("a").DefaultBranch())
	assert.Equal(t, "master", g.Project("b").DefaultBranch())
}

func TestTheStewardOfAProject(t *testing.T) {
	tests := []struct {
		name, repo, want string
	}{
		{"managedBy as given", "{name: acme/a, managedBy: release-team}", "release-team"},
		{"an owned repo is orbs'", "{name: acme/a, owned: true}", "orbs"},
		{"an owned repo says who", "{name: acme/a, owned: true, managedBy: shared}", "shared"},
		{"an unowned repo has none", "{name: acme/a}", ""},
		{"an external repo has none", "{name: acme/a, external: true}", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := chaintest.Load(t, project("a", "  repo: "+tt.repo+"\n"))

			assert.Equal(t, tt.want, g.Project("a").Steward())
		})
	}
}

const perLine = `  repo: {name: acme/a}
  branches:
    layout: perLine
    stages: [dev, stable]
    lines:
      - {line: v2.16}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: ""}
      - {line: v2.13, branch: {dev: dev-v2.13, stable: release-v2.13}}
`

func TestTheBranchOfALine(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))
	a, s := g.Project("a"), g.Project("s")

	tests := []struct {
		name, line, stage string
		want              string
		ok                bool
	}{
		{"a line in development with no branch written", "v2.16", "", "main", true},
		{"a branch", "v2.15", "", "release/v2.15", true},
		{"a line without its v", "2.15", "", "release/v2.15", true},
		{"a line in capitals", "V2.15", "", "release/v2.15", true},
		{"one branch serves every stage", "v2.15", "dev", "release/v2.15", true},
		{"no branch yet", "v2.14", "", "", false},
		{"a branch for each stage", "v2.13", "dev", "dev-v2.13", true},
		{"the other stage", "v2.13", "stable", "release-v2.13", true},
		{"a stage map with no stage asked", "v2.13", "", "", false},
		{"a stage that has no branch", "v2.13", "beta", "", false},
		{"a line the project does not list", "v2.99", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := a.BranchOfLine(tt.line, tt.stage)

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.ok, ok)
		})
	}

	t.Run("a single layout has the default branch for every line", func(t *testing.T) {
		got, ok := s.BranchOfLine("v2.99", "")

		assert.True(t, ok)
		assert.Equal(t, "main", got)
	})
}

func TestTheLinesAProjectLists(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, []string{"v2.16", "v2.15", "v2.14", "v2.13"}, g.Project("a").Lines())
	assert.Empty(t, g.Project("s").Lines())
}

func TestTheLineOfABranch(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))
	a := g.Project("a")

	assert.Equal(t, "v2.16", a.LineOfBranch("main"))
	assert.Equal(t, "v2.15", a.LineOfBranch("release/v2.15"))
	assert.Equal(t, "v2.13", a.LineOfBranch("dev-v2.13"))
	assert.Equal(t, "v2.13", a.LineOfBranch("release-v2.13"))
	assert.Empty(t, a.LineOfBranch("feature/x"))
	assert.Empty(t, a.LineOfBranch(""))
	assert.Empty(t, g.Project("s").LineOfBranch("main"), "a single layout's branch holds every line")
}

func TestTheReleaseBranchesOfAProject(t *testing.T) {
	g := chaintest.Load(t, chaintest.Lines("v2.16", "v2.15", "v2.14", "v2.13"), project("a", perLine), project("s", "  repo: {name: acme/s}\n"))

	assert.Equal(t, []string{"main", "release/v2.15", "dev-v2.13", "release-v2.13"}, g.Project("a").ReleaseBranches())
	assert.Equal(t, []string{"main"}, g.Project("s").ReleaseBranches())
}

func TestAProjectIsOwnedOrExternal(t *testing.T) {
	g := chaintest.Load(t,
		project("mine", "  repo: {name: acme/mine, owned: true}\n"),
		project("theirs", "  repo: {name: acme/theirs, external: true}\n"),
		project("plain", "  repo: {name: acme/plain}\n"),
	)

	assert.True(t, g.Project("mine").Owned())
	assert.False(t, g.Project("mine").External())
	assert.False(t, g.Project("theirs").Owned())
	assert.True(t, g.Project("theirs").External())
	assert.False(t, g.Project("plain").Owned())
	assert.False(t, g.Project("plain").External())
}
