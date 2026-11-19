package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTeams(t *testing.T, spec string) (*Teams, bool, func(path, want string) bool) {
	t.Helper()
	objs, rep := decodeReport(t, "apiVersion: gero/v1alpha2\nkind: Teams\nname: rancher\nspec:\n"+spec)
	var ts *Teams
	if len(objs) > 0 {
		require.Len(t, objs, 1)
		ts = objs[0].(*Teams)
	}
	return ts, rep.Err() == nil, func(path, want string) bool { return hasError(rep, path, want) }
}

func TestAValidTeamsHasNoFindings(t *testing.T) {
	ts, ok, _ := loadTeams(t, `  teams:
    - {id: orbs, github: orbs}
    - {id: frameworks, github: rancher-squad-frameworks}
    - {id: hostbusters, github: rancher-team-2-hostbusters-dev}
    - {id: scc, github: "@SUSE/scc-team"}
    - {id: qa, github: orbs_qa}
`)

	assert.True(t, ok)
	require.NotNil(t, ts)
	assert.Len(t, ts.Spec.Teams, 5)
}

func TestTeamsRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"no id", "  teams: [{github: orbs}]\n", "spec.teams[0].id", "required"},
		{"an id that is not a slug", "  teams: [{id: Orbs Team, github: orbs}]\n", "spec.teams[0].id", "lowercase slug"},
		{"an id of shared, which is not a team", "  teams: [{id: shared, github: orbs}]\n", "spec.teams[0].id", "shared"},
		{"an id listed twice", "  teams: [{id: orbs, github: orbs}, {id: orbs, github: other}]\n", "spec.teams[1].id", "twice"},
		{"no github team", "  teams: [{id: orbs}]\n", "spec.teams[0].github", "required"},
		{"a github team with a space", "  teams: [{id: orbs, github: \"or bs\"}]\n", "spec.teams[0].github", "GitHub team"},
		{"a github team written as org/team without the @", "  teams: [{id: orbs, github: SUSE/scc-team}]\n", "spec.teams[0].github", "@org/team"},
		{"a github team of an org with no team", "  teams: [{id: orbs, github: \"@SUSE/\"}]\n", "spec.teams[0].github", "@org/team"},
		{"a github team with a second slash", "  teams: [{id: orbs, github: \"@SUSE/a/b\"}]\n", "spec.teams[0].github", "@org/team"},
		{"a github team listed under two ids", "  teams: [{id: orbs, github: orbs}, {id: o, github: \"@rancher/orbs\"}]\n", "spec.teams[1].github", "already"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _, hasErr := loadTeams(t, tt.spec)

			assert.Nil(t, ts, "a Teams that breaks a rule is dropped")
			assert.True(t, hasErr(tt.path, tt.want), "want an error at %s containing %q", tt.path, tt.want)
		})
	}
}
