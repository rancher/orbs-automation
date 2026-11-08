package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectSteward(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want string
	}{
		{"owned and unset is orbs", Repo{Owned: true}, "orbs"},
		{"owned by another team's name", Repo{Owned: true, ManagedBy: "release-team"}, "release-team"},
		{"unowned and unset is unspecified", Repo{}, ""},
		{"unowned and orbs written out", Repo{ManagedBy: "orbs"}, "orbs"},
		{"unowned and another team", Repo{ManagedBy: "release-team"}, "release-team"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Project{Repo: tt.repo}.Steward())
		})
	}
}

func TestTagSchemeStages(t *testing.T) {
	assert.Equal(t, []string{"dev", "stable"}, TagSchemeStages(TagSchemeCalverLineStage))
	assert.Empty(t, TagSchemeStages(TagSchemeSemver), "semver tags carry no stage")
	assert.Empty(t, TagSchemeStages("nonsense"))
}
