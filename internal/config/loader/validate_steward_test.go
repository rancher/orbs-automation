package loader

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observed is withMethods with the producer b made like rancher-assets: not
// owned, managed by the release team, releases on one branch with its lines in
// CalVer tags, and an edge that follows the release stage.
func observed() config.Config {
	c := withMethods()
	b := &c.Projects[1]
	b.Repo.Owned, b.Repo.ManagedBy = false, "release-team"
	b.Release.Branching, b.Release.TagScheme = config.BranchingPerLineTag, config.TagSchemeCalverLineStage
	b.Release.Lines = []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}} // v2.15 is ga, and needs no branch here
	b.Downstream[0].Stage = config.StageRelease
	return c
}

func TestValidateAcceptsAnObservedProducer(t *testing.T) {
	assert.NoError(t, Validate(WithDefaults(observed())))
}

func TestValidateAcceptsAnObservedEdgeWithNoStage(t *testing.T) {
	c := observed()
	c.Projects[1].Downstream[0].Stage = ""

	assert.NoError(t, Validate(WithDefaults(c)))
}

func TestValidateStewardshipAndTagProblems(t *testing.T) {
	producer := func(c *config.Config) *config.Project { return &c.Projects[1] }
	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   []string
	}{
		{"managedBy with a capital", func(c *config.Config) { producer(c).Repo.ManagedBy = "Release-Team" }, []string{`Project "b"`, "repo.managedBy", `"Release-Team"`}},
		{"managedBy with a space", func(c *config.Config) { producer(c).Repo.ManagedBy = "release team" }, []string{"repo.managedBy"}},
		{"perLineTag with no lines", func(c *config.Config) {
			producer(c).Release.Lines = nil
		}, []string{`Project "b"`, "release.lines", "perLineTag"}},
		{"perLineTag line with a branch", func(c *config.Config) {
			producer(c).Release.Lines[0].Branch = "release/v2.16"
		}, []string{"release.lines[0]", "branch", "perLineTag"}},
		{"perLineTag line with a version", func(c *config.Config) {
			producer(c).Release.Lines[1].Version = "1.x"
		}, []string{"release.lines[1]", "version", "perLineTag"}},
		{"perLineTag with the default scheme", func(c *config.Config) {
			producer(c).Release.TagScheme = config.TagSchemeSemver
		}, []string{"release.tagScheme", "perLineTag", "calverLineStage"}},
		{"unknown tagScheme", func(c *config.Config) { producer(c).Release.TagScheme = "weekly" }, []string{"release.tagScheme", `"weekly"`}},
		{"calver tags without perLineTag", func(c *config.Config) {
			producer(c).Release.Branching = config.BranchingSingle
		}, []string{"release.tagScheme", "calverLineStage", "perLineTag"}},
		{"stage on an owned producer's edge", func(c *config.Config) { producer(c).Repo.Owned = true }, []string{"downstream[0]", "stage", "owned"}},
		{"stage the scheme does not give", func(c *config.Config) {
			producer(c).Downstream[0].Stage = "stable"
		}, []string{"downstream[0]", "stage", `"stable"`, "dev, release"}},
		{"stage under a scheme with none", func(c *config.Config) {
			producer(c).Release.Branching, producer(c).Release.TagScheme = config.BranchingSingle, config.TagSchemeSemver
		}, []string{"downstream[0]", "stage"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := observed()
			tt.mutate(&c)

			err := Validate(WithDefaults(c))

			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}
