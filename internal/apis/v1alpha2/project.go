package v1alpha2

// Project is a repo as a place: who owns it, how its pull requests look, and
// which branch each Rancher line is on.
type Project struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       ProjectSpec `yaml:"spec"`
}

// ProjectSpec is the fields of a Project.
type ProjectSpec struct {
	Repo             Repo         `yaml:"repo"`
	FinalDestination bool         `yaml:"finalDestination"`
	PullRequests     PullRequests `yaml:"pullRequests"`
	Branches         Branches     `yaml:"branches"`
}

// Repo identifies the repository.
type Repo struct {
	Name          string `yaml:"name"` // owner/name
	Owned         bool   `yaml:"owned"`
	External      bool   `yaml:"external"` // outside our organization
	ManagedBy     string `yaml:"managedBy"`
	DefaultBranch string `yaml:"defaultBranch"`
}

// PullRequests holds the conventions for PRs opened against the repo.
type PullRequests struct {
	Title        string   `yaml:"title"`
	BranchPrefix string   `yaml:"branchPrefix"`
	Labels       []string `yaml:"labels"`
	BotLabels    []string `yaml:"botLabels"`
	PushTo       string   `yaml:"pushTo"`
}

// Branches is the repo's branch layout.
type Branches struct {
	Layout string       `yaml:"layout"` // single or perLine
	Stages []string     `yaml:"stages"` // for a layout whose stages are branches
	Lines  []BranchLine `yaml:"lines"`
}

// BranchLine is the branch of one Rancher line.
type BranchLine struct {
	Line string `yaml:"line"`
	// Branch is nil when the field is omitted, which means the default branch for
	// a line in development.
	Branch *Branch `yaml:"branch"`
}

// Branch is a line's branch in the forms it can be written: a string, an empty
// string (the line has no branch yet), or a map of stage to branch.
type Branch struct {
	Name   string            // a single branch; empty with no Stages means no branch yet
	Stages map[string]string // a branch for each stage
}
