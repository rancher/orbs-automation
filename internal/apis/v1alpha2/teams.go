package v1alpha2

// KindTeams is the kind of the one document that lists the teams.
const KindTeams = "Teams"

// Teams is the set of teams that manage Projects: each an id gero uses, and the
// GitHub team it stands for. It is the only place a GitHub team name is written.
type Teams struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       TeamsSpec `yaml:"spec"`
}

// TeamsSpec is the fields of Teams.
type TeamsSpec struct {
	Teams []Team `yaml:"teams"`
}

// Team is one team: the id every other place names it by, and its GitHub team,
// bare for a team of the rancher organization or @org/team for another.
type Team struct {
	ID     string `yaml:"id"`
	GitHub string `yaml:"github"`
}
