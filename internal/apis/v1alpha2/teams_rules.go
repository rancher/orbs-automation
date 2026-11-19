package v1alpha2

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// DefaultOrg is the GitHub organization of a team written bare.
const DefaultOrg = "rancher"

var (
	// bareTeam is a GitHub team name without its organization.
	bareTeam = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	// qualifiedTeam is @org/team.
	qualifiedTeam = regexp.MustCompile(`^@[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

// Qualified is the GitHub team as @org/team, in lowercase: a bare team is of
// DefaultOrg.
func (t Team) Qualified() string {
	if strings.HasPrefix(t.GitHub, "@") {
		return strings.ToLower(t.GitHub)
	}
	return strings.ToLower("@" + DefaultOrg + "/" + t.GitHub)
}

// SetDefaults does nothing: a team has no default.
func (t *Teams) SetDefaults() {}

// Validate checks each team: an id that is a slug and not shared, listed once, and
// a GitHub team that is bare or @org/team, which no other team stands for.
func (t *Teams) Validate(_ config.Context, r *config.Report) {
	ids := map[string]bool{}
	github := map[string]string{}
	for i, team := range t.Spec.Teams {
		at := fmt.Sprintf("spec.teams[%d]", i)
		switch {
		case team.ID == "":
			r.Errorf(at+".id", "required")
		case !slug.MatchString(team.ID):
			r.Errorf(at+".id", "%q must be a lowercase slug such as release", team.ID)
		case team.ID == ManagedByShared:
			r.Errorf(at+".id", "%s is every team, so it cannot be the id of one", ManagedByShared)
		case ids[team.ID]:
			r.Errorf(at+".id", "team %s is listed twice", team.ID)
		}
		ids[team.ID] = true
		switch {
		case team.GitHub == "":
			r.Errorf(at+".github", "required")
		case strings.ContainsAny(team.GitHub, "@/") && !qualifiedTeam.MatchString(team.GitHub):
			r.Errorf(at+".github", "%q must be a bare team of %s or @org/team", team.GitHub, DefaultOrg)
		case !strings.ContainsAny(team.GitHub, "@/") && !bareTeam.MatchString(team.GitHub):
			r.Errorf(at+".github", "%q is not a GitHub team name", team.GitHub)
		default:
			if other, dup := github[team.Qualified()]; dup {
				r.Errorf(at+".github", "%s is already the GitHub team of %s", team.GitHub, other)
			}
			github[team.Qualified()] = team.ID
		}
	}
}
