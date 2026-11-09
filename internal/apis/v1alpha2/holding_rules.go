package v1alpha2

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// What a Holding may hold, the formats it may read, how much it accepts, and the
// defaults of a Holding.
const (
	HoldsRelease = "release"
	HoldsCommit  = "commit"
	HoldsBranch  = "branch"

	FormatYAML       = "yaml"
	FormatDockerfile = "dockerfile"
	FormatGoMod      = "gomod"
	FormatRegex      = "regex"

	AcceptStable = "stable"
	AcceptAll    = "all"

	DefaultForm = "{tag}"
)

// atVars are the variables a file and a key may use, filled for each line and
// stage the holding is read for.
var atVars = []string{"line", "stage", "branch"}

// formVars are the variables a form may use, filled from a release and its
// artifact.
var formVars = []string{"image", "tag", "prefix", "bareTag", "version", "url", "file", "sourceRepo", "commit", "base", "upstream"}

// SetDefaults fills the format, the accept and, for a release, the form.
func (h *Holding) SetDefaults() {
	for i := range h.Spec.At {
		if h.Spec.At[i].Format == "" {
			h.Spec.At[i].Format = FormatYAML
		}
	}
	if h.Spec.Holds.Accept == "" {
		h.Spec.Holds.Accept = AcceptStable
	}
	if h.Spec.Holds.Kind == HoldsRelease && h.Spec.Holds.Form == "" {
		h.Spec.Holds.Form = DefaultForm
	}
}

// Validate checks what a Holding can alone: where to look, what it holds, and how
// the text is written. That its references name the right documents is checked
// across documents.
func (h *Holding) Validate(_ config.Context, r *config.Report) {
	holds := h.Spec.Holds

	if len(h.Spec.At) == 0 {
		r.Errorf("spec.at", "is required")
	}
	for i, at := range h.Spec.At {
		h.validateLocation(r, atPath(i, len(h.Spec.At)), at)
	}
	h.validateOverlap(r)

	kindOK := oneOf(r, "spec.holds.kind", holds.Kind, HoldsRelease, HoldsCommit, HoldsBranch)
	if kindOK {
		h.validateForm(r)
		if holds.Kind == HoldsRelease && len(holds.Stages) > 0 {
			r.Errorf("spec.holds.stages", "are for a commit or a branch: a release is found on one stage, named by holds.stage")
		}
	}
	for i, st := range holds.Stages {
		if st == "" {
			r.Errorf(fmt.Sprintf("spec.holds.stages[%d]", i), "is empty")
		}
	}
	oneOf(r, "spec.holds.accept", holds.Accept, AcceptStable, AcceptAll)
	checkRange(r, "spec.holds.lines", holds.Lines)
}

// atPath is where a location is written: spec.at for the only one, and
// spec.at[i] for one of several.
func atPath(i, n int) string {
	if n == 1 {
		return "spec.at"
	}
	return fmt.Sprintf("spec.at[%d]", i)
}

func (h *Holding) validateLocation(r *config.Report, base string, at At) {
	holds := h.Spec.Holds
	for path, text := range map[string]string{base + ".file": at.File, base + ".key": at.Key} {
		if text == "" {
			r.Errorf(path, "is required")
			continue
		}
		if err := templates.Validate(text, atVars); err != nil {
			r.Errorf(path, "%v", err)
			continue
		}
		if uses, _ := templates.Placeholders(text); slices.Contains(uses, "stage") && holds.Stage == "" && len(holds.Stages) == 0 {
			r.Errorf(path, "reads {stage}, so holds.stage or holds.stages must say which stage")
		}
	}
	oneOf(r, base+".format", at.Format, FormatYAML, FormatDockerfile, FormatGoMod, FormatRegex)
	if at.Format == FormatRegex {
		checkPattern(r, base+".key", at.Key)
	}
	checkRange(r, base+".lines", at.Lines)
}

// validateOverlap reports a place written twice for lines that share a line: two
// locations of one holding at the same file and key must be for different lines.
func (h *Holding) validateOverlap(r *config.Report) {
	locations := h.Spec.At
	for i := 1; i < len(locations); i++ {
		for j := range i {
			a, b := locations[j], locations[i]
			if a.File != b.File || a.Key != b.Key || !a.Lines.Overlaps(b.Lines) {
				continue
			}
			r.Errorf(atPath(i, len(locations)), "is the same place as spec.at[%d] (%s, %s) for lines that overlap", j, b.File, b.Key)
			break
		}
	}
}

// checkPattern reports a key in the regex format that is not a pattern with exactly
// one capture group. The key is a template, so it is rendered with sample values
// first, as the engine renders it with the real ones.
func checkPattern(r *config.Report, path, key string) {
	if key == "" || templates.Validate(key, atVars) != nil {
		return // said already
	}
	rendered, err := templates.Render(key, map[string]string{"line": "v0.0", "stage": "stage", "branch": "branch"})
	if err != nil {
		return
	}
	re, err := regexp.Compile("(?m)" + rendered)
	switch {
	case err != nil:
		r.Errorf(path, "is not a pattern: %v", err)
	case re.NumSubexp() != 1:
		r.Errorf(path, "must have exactly one capture group, the value to read and replace, and has %d", re.NumSubexp())
	}
}

func (h *Holding) validateForm(r *config.Report) {
	holds := h.Spec.Holds
	if holds.Kind != HoldsRelease {
		if holds.Form != "" {
			r.Errorf("spec.holds.form", "is only for a release: a %s is held as the text it is", holds.Kind)
		}
		return
	}
	if err := templates.Validate(holds.Form, formVars); err != nil {
		r.Errorf("spec.holds.form", "%v", err)
		return
	}
	if err := templates.CheckReversible(holds.Form); err != nil {
		r.Errorf("spec.holds.form", "%v", err)
	}
}
