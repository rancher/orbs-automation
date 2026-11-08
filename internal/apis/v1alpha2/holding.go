package v1alpha2

import "github.com/rancher/orbs-automation/internal/apis/lines"

// Holding is a named rule for where a piece of information lives in a repo, and
// what it holds of a producer.
type Holding struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       HoldingSpec `yaml:"spec"`
}

// HoldingSpec is the fields of a Holding.
type HoldingSpec struct {
	At    At    `yaml:"at"`
	Holds Holds `yaml:"holds"`
}

// At is where to look.
type At struct {
	File   string `yaml:"file"`   // a template
	Key    string `yaml:"key"`    // a template, then a path in the format's own style
	Format string `yaml:"format"` // yaml or dockerfile
}

// Holds is what the place holds of a producer.
type Holds struct {
	Kind   string      `yaml:"kind"` // release, commit or branch
	Form   string      `yaml:"form"` // a template: how the text is written
	Lines  lines.Range `yaml:"lines"`
	Stage  string      `yaml:"stage"`
	Stages []string    `yaml:"stages"`
	Accept string      `yaml:"accept"` // stable or all
}
