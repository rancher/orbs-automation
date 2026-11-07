package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectBranch(t *testing.T) {
	p := Project{Repo: Repo{Name: "o/a", DefaultBranch: "dev-v2.13"}}

	t.Run("a named branch is used as written", func(t *testing.T) {
		got, isDefault := p.Branch(ProjectLine{Line: "v2.12", Branch: "release/v2.12"})
		assert.Equal(t, "release/v2.12", got)
		assert.False(t, isDefault)
	})
	t.Run("no branch means the default branch", func(t *testing.T) {
		got, isDefault := p.Branch(ProjectLine{Line: "v2.13"})
		assert.Equal(t, "dev-v2.13", got)
		assert.True(t, isDefault)
	})
	t.Run("the raw value stays empty", func(t *testing.T) {
		l := ProjectLine{Line: "v2.13"}
		p.Branch(l)
		assert.Empty(t, l.Branch)
	})
}

func TestProjectIsUniversal(t *testing.T) {
	assert.True(t, Project{}.IsUniversal(), "no lines is the default")
	assert.False(t, Project{Release: Release{Lines: []ProjectLine{{Line: "v2.12"}}}}.IsUniversal())
}

func TestProjectReleaseBranches(t *testing.T) {
	repo := Repo{Name: "o/a", DefaultBranch: "main"}
	lines := []ProjectLine{
		{Line: "v2.16"}, // development: the default branch
		{Line: "v2.15", Branch: "release/v2.15"},
		{Line: "v2.14", Branch: "release/v2.14"},
		{Line: "v2.13", Branch: "main"}, // the same branch as the development line
	}

	t.Run("single is the default branch, whatever the lines say", func(t *testing.T) {
		p := Project{Repo: repo, Release: Release{Branching: BranchingSingle, Lines: lines}}
		assert.Equal(t, []string{"main"}, p.ReleaseBranches())
	})
	t.Run("perLine is each line's effective branch, once each", func(t *testing.T) {
		p := Project{Repo: repo, Release: Release{Branching: BranchingPerLine, Lines: lines}}
		assert.Equal(t, []string{"main", "release/v2.15", "release/v2.14"}, p.ReleaseBranches())
	})
	t.Run("perLine with no lines is empty", func(t *testing.T) {
		p := Project{Repo: repo, Release: Release{Branching: BranchingPerLine}}
		assert.Empty(t, p.ReleaseBranches())
	})
}

func TestProjectLineSeries(t *testing.T) {
	assert.Equal(t, "9.x", ProjectLine{Line: "v2.16", Version: "9.x"}.Series(), "a written version is the series")
	assert.Equal(t, "v2.15", ProjectLine{Line: "v2.15"}.Series(), "else the line is")
}

func TestProjectLineOf(t *testing.T) {
	rancher := Project{Release: Release{Lines: []ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}}}}
	kuberlr := Project{Release: Release{Lines: []ProjectLine{{Line: "v2.16", Version: "9.x"}, {Line: "v2.15", Version: "8.x"}}}}

	assert.Equal(t, "v2.15", rancher.LineOf("v2.15.3"))
	assert.Equal(t, "v2.16", rancher.LineOf("v2.16.0-rc.1"), "a prerelease is in its line")
	assert.Equal(t, "", rancher.LineOf("v2.14.1"), "a line not in config")
	assert.Equal(t, "", rancher.LineOf("main-abc1234"), "not a version")
	assert.Equal(t, "v2.15", kuberlr.LineOf("v8.1.2"), "through the series")
	assert.Equal(t, "v2.16", kuberlr.LineOf("v9.0.0"))
	assert.Equal(t, "", Project{}.LineOf("v1.0.0"), "no lines")
}

func TestProjectBranchOfLine(t *testing.T) {
	repo := Repo{Name: "o/a", DefaultBranch: "main"}
	lines := []ProjectLine{{Line: "v2.16"}, {Line: "v2.15", Branch: "release/v2.15"}}
	perLine := Project{Repo: repo, Release: Release{Branching: BranchingPerLine, Lines: lines}}
	single := Project{Repo: repo, Release: Release{Branching: BranchingSingle, Lines: lines}}

	for line, want := range map[string]string{"v2.15": "release/v2.15", "2.15": "release/v2.15", "v2.16": "main"} {
		got, ok := perLine.BranchOfLine(line)
		assert.True(t, ok, line)
		assert.Equal(t, want, got, "perLine: the line's own branch, else the default: "+line)
	}
	got, ok := single.BranchOfLine("v2.15")
	assert.True(t, ok)
	assert.Equal(t, "main", got, "single keeps every line on the default branch")
	_, ok = perLine.BranchOfLine("v2.99")
	assert.False(t, ok, "a line the project does not have")
	_, ok = Project{Repo: repo}.BranchOfLine("v2.15")
	assert.False(t, ok, "no lines at all")
}

func TestProjectIsNotUniversalWhenItsLinesAreInItsTags(t *testing.T) {
	p := Project{Release: Release{Branching: BranchingPerLineTag}}

	assert.False(t, p.IsUniversal(), "a perLineTag project lists no lines and still follows them")
	assert.Equal(t, AcceptLinesAll, Edge{}.AcceptLines(p), "so an edge from it maps each line to the consumer's branch for it")
}

func TestAPerLineTagProjectKeepsEveryLineOnItsDefaultBranch(t *testing.T) {
	p := Project{
		Repo:    Repo{Name: "o/assets", DefaultBranch: "main"},
		Release: Release{Branching: BranchingPerLineTag, Lines: []ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}}},
	}

	branch, ok := p.BranchOfLine("v2.15")
	assert.True(t, ok)
	assert.Equal(t, "main", branch)
	assert.Equal(t, []string{"main"}, p.ReleaseBranches(), "one branch, whatever lines it lists")
}
