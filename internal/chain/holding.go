package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Holding is a place in the holder's repo where a piece of information about a
// producer lives.
type Holding struct {
	Doc *v1alpha2.Holding
}

// Name is the name of the document.
func (h *Holding) Name() string { return "" }

// Kind is what the holding holds: release, commit or branch.
func (h *Holding) Kind() string { return "" }

// Holder is the project whose file it is.
func (h *Holding) Holder() *Project { return nil }

// Producer is the project the holding holds something of: the one whose stream
// makes the artifact, or the one a commit or a branch is taken from.
func (h *Holding) Producer() *Project { return nil }

// Artifact is the artifact a release holding holds, and nil for a commit or a
// branch.
func (h *Holding) Artifact() *Artifact { return nil }

// AppliesTo reports whether the holding is for the line: its own range holds it,
// the holder lists it (a single layout lists every line), and the producer lists
// it (a stream serves it, and for a release the artifact applies to it).
func (h *Holding) AppliesTo(_ string) bool { return false }

// Edits returns the edits that follow the holding, in the order read.
func (h *Holding) Edits() []*Edit { return nil }
