package loader

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestWithDefaultsFillsOmittedFields(t *testing.T) {
	c := config.Config{
		Projects: []config.Project{{
			Name:    "a",
			Repo:    config.Repo{Name: "o/a"},
			Sources: []config.Source{{From: "b"}},
		}},
		Externals: []config.External{{Name: "x", Repo: config.Repo{Name: "t/x"}}},
	}

	got := WithDefaults(c)

	assert.Equal(t, config.Config{
		Projects: []config.Project{{
			Name:    "a",
			Repo:    config.Repo{Name: "o/a", DefaultBranch: "main"},
			Release: config.Release{Branches: "single", TagPattern: "v*"},
			Sources: []config.Source{{From: "b", Accept: config.Accept{When: "stable", Lines: "all"}}},
		}},
		Externals: []config.External{{Name: "x", Repo: config.Repo{Name: "t/x", DefaultBranch: "main"}}},
	}, got)
}

func TestWithDefaultsKeepsExplicitValues(t *testing.T) {
	c := config.Config{Projects: []config.Project{{
		Name:    "a",
		Repo:    config.Repo{Name: "o/a", Owned: true, DefaultBranch: "dev"},
		Release: config.Release{Branches: "perLine", TagPattern: "v2.*", RequiresPrerelease: true},
		Sources: []config.Source{{From: "b", Accept: config.Accept{When: "all", Lines: "universal"}}},
	}}}

	assert.Equal(t, c, WithDefaults(c))
}

func TestWithDefaultsFillsEachSourceAndKeepsPartialAccept(t *testing.T) {
	c := config.Config{Projects: []config.Project{{
		Name: "a",
		Sources: []config.Source{
			{From: "b", Accept: config.Accept{When: "all"}},
			{From: "c", Accept: config.Accept{Lines: "universal"}},
		},
	}}}

	got := WithDefaults(c).Projects[0].Sources

	assert.Equal(t, config.Accept{When: "all", Lines: "all"}, got[0].Accept)
	assert.Equal(t, config.Accept{When: "stable", Lines: "universal"}, got[1].Accept)
}

func TestWithDefaultsDoesNotModifyReceiver(t *testing.T) {
	c := config.Config{
		Projects:  []config.Project{{Name: "a", Sources: []config.Source{{From: "b"}}}},
		Externals: []config.External{{Name: "x"}},
	}

	_ = WithDefaults(c)

	assert.Equal(t, config.Config{
		Projects:  []config.Project{{Name: "a", Sources: []config.Source{{From: "b"}}}},
		Externals: []config.External{{Name: "x"}},
	}, c)
}

func TestWithDefaultsLeavesLinesAndEmptyConfigAlone(t *testing.T) {
	assert.Equal(t, config.Config{}, WithDefaults(config.Config{}))

	lines := &config.RancherLines{Name: "rancher", Lines: []config.Line{{ID: "v2.16", Phase: "development"}}}

	assert.Equal(t, lines, WithDefaults(config.Config{Lines: lines}).Lines)
}
