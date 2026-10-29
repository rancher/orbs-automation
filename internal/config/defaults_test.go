package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithDefaultsFillsOmittedFields(t *testing.T) {
	c := Config{
		Projects: []Project{{
			Name:    "a",
			Repo:    Repo{Name: "o/a"},
			Sources: []Source{{From: "b"}},
		}},
		Externals: []External{{Name: "x", Repo: Repo{Name: "t/x"}}},
	}

	got := c.WithDefaults()

	assert.Equal(t, Config{
		Projects: []Project{{
			Name:    "a",
			Repo:    Repo{Name: "o/a", DefaultBranch: "main"},
			Release: Release{Branches: "single", TagPattern: "v*"},
			Sources: []Source{{From: "b", Accept: Accept{When: "stable", Lines: "all"}}},
		}},
		Externals: []External{{Name: "x", Repo: Repo{Name: "t/x", DefaultBranch: "main"}}},
	}, got)
}

func TestWithDefaultsKeepsExplicitValues(t *testing.T) {
	c := Config{Projects: []Project{{
		Name:    "a",
		Repo:    Repo{Name: "o/a", Owned: true, DefaultBranch: "dev"},
		Release: Release{Branches: "perLine", TagPattern: "v2.*", RequiresPrerelease: true},
		Sources: []Source{{From: "b", Accept: Accept{When: "all", Lines: "universal"}}},
	}}}

	assert.Equal(t, c, c.WithDefaults())
}

func TestWithDefaultsFillsEachSourceAndKeepsPartialAccept(t *testing.T) {
	c := Config{Projects: []Project{{
		Name: "a",
		Sources: []Source{
			{From: "b", Accept: Accept{When: "all"}},
			{From: "c", Accept: Accept{Lines: "universal"}},
		},
	}}}

	got := c.WithDefaults().Projects[0].Sources

	assert.Equal(t, Accept{When: "all", Lines: "all"}, got[0].Accept)
	assert.Equal(t, Accept{When: "stable", Lines: "universal"}, got[1].Accept)
}

func TestWithDefaultsDoesNotModifyReceiver(t *testing.T) {
	c := Config{
		Projects:  []Project{{Name: "a", Sources: []Source{{From: "b"}}}},
		Externals: []External{{Name: "x"}},
	}

	_ = c.WithDefaults()

	assert.Equal(t, Config{
		Projects:  []Project{{Name: "a", Sources: []Source{{From: "b"}}}},
		Externals: []External{{Name: "x"}},
	}, c)
}

func TestWithDefaultsLeavesLinesAndEmptyConfigAlone(t *testing.T) {
	assert.Equal(t, Config{}, Config{}.WithDefaults())

	lines := &RancherLines{Name: "rancher", Lines: []Line{{ID: "v2.16", Phase: "development"}}}

	assert.Equal(t, lines, Config{Lines: lines}.WithDefaults().Lines)
}
