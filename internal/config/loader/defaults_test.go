package loader

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestWithDefaultsFillsOmittedFields(t *testing.T) {
	c := config.Config{
		Projects: []config.Project{{
			Name:          "a",
			Repo:          config.Repo{Name: "o/a"},
			UpdateMethods: []config.UpdateMethod{{Name: "m"}},
		}},
		Externals: []config.External{{Name: "x", Repo: config.Repo{Name: "t/x"}}},
	}

	got := WithDefaults(c)

	assert.Equal(t, config.Config{
		Projects: []config.Project{{
			Name:          "a",
			Repo:          config.Repo{Name: "o/a", DefaultBranch: "main"},
			Release:       config.Release{Branches: "single", TagPattern: "v*"},
			UpdateMethods: []config.UpdateMethod{{Name: "m", Accept: config.Accept{When: "stable"}}},
		}},
		Externals: []config.External{{Name: "x", Repo: config.Repo{Name: "t/x", DefaultBranch: "main"}}},
	}, got)
}

func TestWithDefaultsKeepsExplicitValues(t *testing.T) {
	c := config.Config{Projects: []config.Project{{
		Name:          "a",
		Repo:          config.Repo{Name: "o/a", Owned: true, DefaultBranch: "dev"},
		Release:       config.Release{Branches: "perLine", TagPattern: "v2.*", RequiresPrerelease: true},
		UpdateMethods: []config.UpdateMethod{{Name: "m", Accept: config.Accept{When: "all"}}},
	}}}

	assert.Equal(t, c, WithDefaults(c))
}

func TestWithDefaultsFillsEachMethodAndKeepsWhatIsWritten(t *testing.T) {
	c := config.Config{Projects: []config.Project{{
		Name: "a",
		UpdateMethods: []config.UpdateMethod{
			{Name: "x", Accept: config.Accept{When: "all"}},
			{Name: "y"},
		},
		Downstream: []config.Edge{{To: "b", Method: "z"}},
	}}}

	got := WithDefaults(c).Projects[0]

	assert.Equal(t, "all", got.UpdateMethods[0].Accept.When)
	assert.Equal(t, "stable", got.UpdateMethods[1].Accept.When)
	assert.Equal(t, c.Projects[0].Downstream, got.Downstream, "an edge's lines are derived from the producer when read, so they are not defaulted")
}

func TestWithDefaultsDoesNotModifyReceiver(t *testing.T) {
	c := config.Config{
		Projects:  []config.Project{{Name: "a", UpdateMethods: []config.UpdateMethod{{Name: "m"}}}},
		Externals: []config.External{{Name: "x"}},
	}

	_ = WithDefaults(c)

	assert.Equal(t, config.Config{
		Projects:  []config.Project{{Name: "a", UpdateMethods: []config.UpdateMethod{{Name: "m"}}}},
		Externals: []config.External{{Name: "x"}},
	}, c)
}

func TestWithDefaultsLeavesLinesAndEmptyConfigAlone(t *testing.T) {
	assert.Equal(t, config.Config{}, WithDefaults(config.Config{}))

	lines := &config.RancherLines{Name: "rancher", Lines: []config.Line{{ID: "v2.16", Phase: "development"}}}

	assert.Equal(t, lines, WithDefaults(config.Config{Lines: lines}).Lines)
}
