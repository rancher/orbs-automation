package chain

import (
	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
)

// Holding is a place in the holder's repo where a piece of information about a
// producer lives.
type Holding struct {
	Doc      *v1alpha2.Holding
	holder   *Project
	producer *Project
	artifact *Artifact
	edits    []*Edit

	virtual  bool
	pinnedBy *Holding   // for a virtual Holding: the Holding whose Artifact pins its Artifact
	pinned   []*Holding // for a Holding: the virtual Holdings its Artifact's pins make
}

// Virtual reports whether the Holding is one a pin makes, and no document says: the
// pinned Artifact in the files of the Holding it comes from (PinnedBy).
func (h *Holding) Virtual() bool { return h.virtual }

// PinnedBy is the Holding a virtual Holding comes from, and nil for any other.
func (h *Holding) PinnedBy() *Holding { return h.pinnedBy }

// Pinned returns the virtual Holdings the pins of the Holding's Artifact make: what a
// run that ships the Artifact writes beside it, in the same files.
func (h *Holding) Pinned() []*Holding { return h.pinned }

// Acknowledgement says whether the holding's document acknowledges a warning of a
// rule, about a line when it has one, and the reason when it does.
func (h *Holding) Acknowledgement(rule, line string) (reason string, ok bool) {
	return v1alpha2.Acknowledges(h.Doc.Acknowledge, rule, line)
}

// RuleLocationsDiffer is the rule of the warning that the locations of a holding
// hold different versions, found by reading a repo.
const RuleLocationsDiffer = v1alpha2.RuleLocationsDiffer

// Name is the name of the document.
func (h *Holding) Name() string { return h.Doc.Name }

// Kind is what the holding holds: release, commit or branch.
func (h *Holding) Kind() string { return h.Doc.Spec.Holds.Kind }

// Holder is the project whose file it is.
func (h *Holding) Holder() *Project { return h.holder }

// Producer is the project the holding holds something of: the one whose stream
// makes the artifact, or the one a commit or a branch is taken from.
func (h *Holding) Producer() *Project { return h.producer }

// Artifact is the artifact a release holding holds, and nil for a commit or a
// branch.
func (h *Holding) Artifact() *Artifact { return h.artifact }

// AppliesTo reports whether the holding is for the line: its own range holds it,
// the holder lists it (a single layout lists every line), and the producer lists
// it (a stream serves it, and for a release the artifact applies to it).
func (h *Holding) AppliesTo(line string) bool {
	if !h.Doc.Spec.Holds.Lines.Contains(line) || !h.holder.lists(line) {
		return false
	}
	if h.artifact != nil {
		return h.artifact.stream.Serves(line) && h.artifact.AppliesTo(line)
	}
	return h.producer.serves(line)
}

// Stage is the stage the holding is for, empty when it is for none in particular.
func (h *Holding) Stage() string { return "" }

// Form is how the holding writes a release, as written: a template over the
// release's variables.
func (h *Holding) Form() string { return "" }

// AcceptsAll reports whether the holding takes every release, a prerelease too,
// and not only a stable one.
func (h *Holding) AcceptsAll() bool { return false }

// HoldsLine reports whether the holding's own range of lines holds the line. It is
// narrower than AppliesTo, which also asks whether the holder and the producer list
// the line.
func (h *Holding) HoldsLine(line string) bool { return false }

// Location is one place to look for what a holding holds: a file and a key in it, in
// a format, for some lines.
type Location struct {
	File, Key, Format string // File and Key are templates, as written
	lines             lines.Range
}

// AppliesTo reports whether the file is there for the line.
func (l Location) AppliesTo(line string) bool { return false }

// Locations returns the places the holding is looked for, in the order written.
func (h *Holding) Locations() []Location { return nil }

// Edits returns the edits that follow the holding, in the order read.
func (h *Holding) Edits() []*Edit { return h.edits }
