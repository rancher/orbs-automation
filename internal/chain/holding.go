package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Holding is a place in the holder's repo where a piece of information about a
// producer lives.
type Holding struct {
	Doc      *v1alpha2.Holding
	holder   *Project
	producer *Project
	artifact *Artifact
	edits    []*Edit
}

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

// Edits returns the edits that follow the holding, in the order read.
func (h *Holding) Edits() []*Edit { return h.edits }
