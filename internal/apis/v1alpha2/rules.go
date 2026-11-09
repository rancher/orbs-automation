package v1alpha2

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// The ids of the rules that raise a warning. An id is stable: it is what a
// document names to acknowledge the warning, so it is not renamed.
const (
	RuleCircularHolding              = "circular-holding"
	RuleDefaultBranchPastDevelopment = "default-branch-past-development"
	RuleAcknowledgementUnused        = "acknowledgement-unused"
	RuleStreamDiffersFromDestination = "stream-differs-from-destination"
	RuleLocationsDiffer              = "locations-differ"
)

// acknowledgeable are the rules whose warnings a document can acknowledge: the
// ones of a document. The unknown key of a user file is not, and neither is an
// acknowledgement that matches nothing.
var acknowledgeable = []string{RuleCircularHolding, RuleDefaultBranchPastDevelopment, RuleStreamDiffersFromDestination}

// slug is what a lowercase slug looks like, as in a team name.
var slug = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// oneOf reports an error at path unless value is one of the allowed, and returns
// whether it was.
func oneOf(r *config.Report, path, value string, allowed ...string) bool {
	if slices.Contains(allowed, value) {
		return true
	}
	r.Errorf(path, "must be %s, got %q", orList(allowed), value)
	return false
}

// orList writes a, b or c.
func orList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// checkLineID reports an error at path unless the value looks like a Rancher line.
func checkLineID(r *config.Report, path, value string) {
	if !lines.Valid(value) {
		r.Errorf(path, "%q is not a Rancher line: write it like v2.16", value)
	}
}

// quoteAll quotes each item, for a message that lists them.
func quoteAll(items []string) string {
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = fmt.Sprintf("%q", it)
	}
	return strings.Join(q, ", ")
}

// checkRange reports an error at path unless the range's ends are Rancher lines
// and the since is not after the until.
func checkRange(r *config.Report, path string, rng lines.Range) {
	if rng.Since != "" && !lines.Valid(rng.Since) {
		r.Errorf(path+".since", "%q is not a Rancher line: write it like v2.16", rng.Since)
	}
	if rng.Until != "" && !lines.Valid(rng.Until) {
		r.Errorf(path+".until", "%q is not a Rancher line: write it like v2.16", rng.Until)
	}
	if !lines.Valid(rng.Since) || !lines.Valid(rng.Until) {
		return
	}
	if lines.Compare(rng.Until, rng.Since) < 0 {
		r.Errorf(path, "since %s is after until %s", rng.Since, rng.Until)
	}
}
