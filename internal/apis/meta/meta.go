package meta

import (
	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// TypeMeta says which shape a document is.
type TypeMeta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

// GetAPIVersion returns the document's apiVersion.
func (t TypeMeta) GetAPIVersion() string { return t.APIVersion }

// GetKind returns the document's kind.
func (t TypeMeta) GetKind() string { return t.Kind }

// ObjectMeta is what identifies a document and connects it to others.
type ObjectMeta struct {
	// Name is unique within the kind, and is how refs and logs name the document.
	Name string `yaml:"name"`
	// Refs holds every reference to another document: names only, and the key
	// says which kind the name is of.
	Refs Refs `yaml:"refs"`
	// Acknowledge says which warnings of this document are expected, and why.
	Acknowledge []Acknowledgement `yaml:"acknowledge"`
}

// Acknowledgement says a warning of a rule on this document is expected, so it is
// not shown as one. The reason is required: it is what a reader of the config is
// told, and what the next person to touch it weighs.
type Acknowledgement struct {
	Rule   string      `yaml:"rule"`   // the id of the rule of the warning
	Reason string      `yaml:"reason"` // why the warning is expected
	Lines  lines.Range `yaml:"lines"`  // only for a warning about these lines; every line when not given
}

// GetName returns the document's name.
func (o ObjectMeta) GetName() string { return o.Name }

// GetRefs returns the document's references.
func (o ObjectMeta) GetRefs() Refs { return o.Refs }

// GetAcknowledge returns the warnings the document says are expected.
func (o ObjectMeta) GetAcknowledge() []Acknowledgement { return o.Acknowledge }

// SetDefaults is the default for a kind that has no defaults of its own: it does
// nothing. A kind defines SetDefaults on itself to fill what it defaults.
func (o ObjectMeta) SetDefaults() {}

// Validate is the default for a kind that has no rules of its own: it reports
// nothing. A kind defines Validate on itself to check what it can alone.
func (o ObjectMeta) Validate(_ config.Context, _ *config.Report) {}

// Refs maps a reference key to the names it points at. It is written as one name
// or a list of names.
type Refs map[string][]string

// Object is a decoded document of any kind. It is also a config.Schema, so the
// engine can default and validate it.
type Object interface {
	GetAPIVersion() string
	GetKind() string
	GetName() string
	GetRefs() Refs
	GetAcknowledge() []Acknowledgement
	config.Schema
}
