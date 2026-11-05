// Package config holds the types of the central application config (Project,
// RancherLines, External) and the getters that turn a raw field into the value
// an operation should act on. It only describes the config: loading,
// defaults and validation are in internal/config/loader, which only
// internal/workspace may use, so a command always gets config that has been
// through them. Shapes are documented in config/README.md.
package config

// APIVersion is the only apiVersion the loader accepts until gero v1.0.0.
const APIVersion = "gero/v1alpha1"

// Document kinds.
const (
	KindProject      = "Project"
	KindRancherLines = "RancherLines"
	KindExternal     = "External"
)

// Config is everything found in one config directory, as written (no defaults
// applied yet).
type Config struct {
	Projects  []Project
	Externals []External
	Lines     *RancherLines // nil when the directory has no RancherLines document
}

// Project is a repo we model.
type Project struct {
	Name         string       `yaml:"-"`
	Repo         Repo         `yaml:"repo"`
	Release      Release      `yaml:"release"`
	PullRequests PullRequests `yaml:"pullRequests"`
	Images       []Image      `yaml:"images"`
	Sources      []Source     `yaml:"sources"`
}

// Repo identifies the project's repository.
type Repo struct {
	Name          string `yaml:"name"` // owner/name slug
	Owned         bool   `yaml:"owned"`
	DefaultBranch string `yaml:"defaultBranch"`
}

// Image is a container image the project publishes, and the registries it goes
// to (the Registry* values; none listed means all of them).
type Image struct {
	Name       string   `yaml:"name"` // repository path in a registry, such as rancher/scc-operator
	Registries []string `yaml:"registries"`
}

// Release is the project's release policy.
type Release struct {
	Branches           string        `yaml:"branches"` // single or perLine
	TagPattern         string        `yaml:"tagPattern"`
	RequiresPrerelease bool          `yaml:"requiresPrerelease"`
	Lines              []ProjectLine `yaml:"lines"`
}

// ProjectLine maps one Rancher line to this project's series and branch.
type ProjectLine struct {
	Line    string `yaml:"line"`
	Version string `yaml:"version"`
	Branch  string `yaml:"branch"`
}

// PullRequests holds the conventions for PRs opened against the project.
type PullRequests struct {
	Title        string   `yaml:"title"`
	BranchPrefix string   `yaml:"branchPrefix"`
	Labels       []string `yaml:"labels"`
}

// Source is a project this one takes updates from.
type Source struct {
	From   string `yaml:"from"`
	Accept Accept `yaml:"accept"`
	Update Update `yaml:"update"`
}

// Accept says which of a source's releases are taken.
type Accept struct {
	When  string `yaml:"when"`  // stable or all
	Lines string `yaml:"lines"` // all or universal
}

// Update says how a source's release is applied.
type Update struct {
	Strategy string            `yaml:"strategy"`
	Params   map[string]string `yaml:"params"`
	PostEdit []string          `yaml:"postEdit"`
}

// RancherLines is the set of Rancher lines and facts about each.
type RancherLines struct {
	Name  string `yaml:"-"`
	Lines []Line `yaml:"lines"`
}

// Line is one Rancher line.
type Line struct {
	ID         string          `yaml:"id"`
	Phase      string          `yaml:"phase"` // optional override: development, ga, eom or eol
	Dates      Dates           `yaml:"dates"`
	Kubernetes KubernetesRange `yaml:"kubernetes"`
}

// KubernetesRange is the supported Kubernetes minor range of a line.
type KubernetesRange struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

// External is a third-party project we track or notify.
type External struct {
	Name          string         `yaml:"-"`
	Repo          Repo           `yaml:"repo"`
	Relationships []Relationship `yaml:"relationships"`
}

// Relationship ties an External to one of our projects.
type Relationship struct {
	Project string `yaml:"project"`
	Action  string `yaml:"action"` // trackOnly, react or notify
}
