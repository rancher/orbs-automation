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
	// FinalDestination marks the one project that releases end in, such as
	// rancher/rancher: the others ship to it, directly or through another project,
	// and it ships to none. Only that project sets it, and at most one project may,
	// so a view of how the lines connect has an end to start from without knowing a
	// project by name.
	FinalDestination bool `yaml:"finalDestination"`
	// UpdateMethods are the ways this project takes an update, which it owns;
	// Downstream are the projects it ships to, with which of their methods.
	UpdateMethods []UpdateMethod `yaml:"updateMethods"`
	Downstream    []Edge         `yaml:"downstream"`
}

// Repo identifies the project's repository.
type Repo struct {
	Name string `yaml:"name"` // owner/name slug
	// Owned says whether gero ships from the repository: its workflows run gero.
	// An edge from an unowned project is only observed.
	Owned bool `yaml:"owned"`
	// ManagedBy is the team responsible for the project, as a lowercase slug. Read
	// it through Project.Steward, which defaults it for an owned project.
	ManagedBy     string `yaml:"managedBy"`
	DefaultBranch string `yaml:"defaultBranch"`
}

// Image is a container image the project publishes, and the registries it goes
// to (the Registry* values; none listed means all of them).
type Image struct {
	Name       string   `yaml:"name"` // repository path in a registry, such as rancher/scc-operator
	Registries []string `yaml:"registries"`
	Since      string   `yaml:"since"` // a Rancher line ID: expected only from this line on; empty means every line
}

// Release is the project's release policy.
type Release struct {
	Branching          string        `yaml:"branching"` // how its release branches are laid out: single (one branch), perLine (one for each Rancher line) or perLineTag (one branch, the lines are in its tags)
	TagScheme          string        `yaml:"tagScheme"` // how its tags spell a line: semver, or another scheme with perLineTag
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
	// BotLabels are applied only to a PR the bot opens, such as origin/bot, and
	// never to one an engineer opened from a workstation.
	BotLabels []string `yaml:"botLabels"`
	// PushTo is where a branch for this project is pushed, fork or upstream, for a
	// repository that is only ever pushed to one way. The user's settings and the
	// --push-to flag come before it.
	PushTo string `yaml:"pushTo"`
}

// UpdateMethod is a named way a project takes an update. The consumer owns it:
// the strategy that reads and writes the value, the values it fixes for it
// (With), the params a producer may pass (AllowedParams), what it accepts, and
// the steps it runs around the edit. Many producers can use one method.
type UpdateMethod struct {
	Name          string            `yaml:"name"`
	Strategy      string            `yaml:"strategy"`
	With          map[string]string `yaml:"with"`
	AllowedParams []AllowedParam    `yaml:"allowedParams"`
	Accept        Accept            `yaml:"accept"`
	PreEdit       []Step            `yaml:"preEdit"`
	PostEdit      []Step            `yaml:"postEdit"`
}

// AllowedParam is a param a method offers a producer: its key, what it means,
// and whether it can be left out.
type AllowedParam struct {
	Key         string `yaml:"key"`
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Required    *bool  `yaml:"required"` // omitted: required unless there is a default
}

// Edge is a producer's wiring to one consumer: which project, which of its
// methods, and the params the method allows. The producer owns it.
type Edge struct {
	To       string            `yaml:"to"`     // the consumer, by config name or owner/name
	Method   string            `yaml:"method"` // one of the consumer's update methods
	Params   map[string]string `yaml:"params"`
	Lines    string            `yaml:"lines"` // how releases map to the consumer's branches; omitted is derived
	Stage    string            `yaml:"stage"` // an observed edge only: the producer's stage to follow; omitted follows whatever the consumer holds
	PreEdit  []Step            `yaml:"preEdit"`
	PostEdit []Step            `yaml:"postEdit"`
}

// Step is a command run around an edit, optionally only for a range of the
// consumer's lines.
type Step struct {
	Run   Command `yaml:"run"`
	Since string  `yaml:"since"` // a line ID: only from this line on
	Until string  `yaml:"until"` // a line ID: only up to and including this line
	From  string  `yaml:"from"`  // an edge's step only: where the program comes from
}

// Command is a program or script path with its arguments. In YAML it is one
// string, the program with no arguments, or a list. It is never split like a
// shell command line.
type Command []string

// UnmarshalYAML reads a string as a one-element command and a list as it is. It
// uses the callback form so this package does not import the YAML library.
func (c *Command) UnmarshalYAML(unmarshal func(any) error) error {
	var one string
	if err := unmarshal(&one); err == nil {
		*c = Command{one}
		return nil
	}
	var list []string
	if err := unmarshal(&list); err != nil {
		return err
	}
	*c = list
	return nil
}

// Accept says which of a producer's releases a method takes.
type Accept struct {
	When string `yaml:"when"` // stable or all
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
