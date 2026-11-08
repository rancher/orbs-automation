package v1alpha2

import (
	"fmt"
	"github.com/rancher/orbs-automation/internal/apis/lines"
	"regexp"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// kubernetesMinor is what a Kubernetes minor looks like in a range.
var kubernetesMinor = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

// SetDefaults does nothing: a line has no default. Its phase is worked out from
// its dates when read.
func (l *RancherLines) SetDefaults() {}

// Validate checks each line: its id, listed once, its phase, the order of its
// dates and its Kubernetes range. A date that has passed is not a finding: the
// phase follows the dates, whatever the time.
func (l *RancherLines) Validate(_ config.Context, r *config.Report) {
	seen := map[string]bool{}
	for i, ln := range l.Spec.Lines {
		at := fmt.Sprintf("spec.lines[%d]", i)
		switch {
		case ln.ID == "":
			r.Errorf(at+".id", "required")
		case !lines.Valid(ln.ID):
			checkLineID(r, at+".id", ln.ID)
		case seen[ln.ID]:
			r.Errorf(at+".id", "line %s is listed twice", ln.ID)
		}
		seen[ln.ID] = true
		if ln.Phase != "" {
			oneOf(r, at+".phase", ln.Phase, lines.PhaseDevelopment, lines.PhaseGA, lines.PhaseEOM, lines.PhaseEOL)
		}
		checkDates(r, at+".dates", ln.Dates)
		checkKubernetes(r, at+".kubernetes", ln.Kubernetes)
	}
}

// checkDates reports dates that are out of order: ga, then eom, then eol, each
// optional.
func checkDates(r *config.Report, path string, d lines.Dates) {
	if !d.EOM.IsZero() && d.GA.IsZero() {
		r.Errorf(path+".eom", "set without ga")
	}
	if !d.EOM.IsZero() && !d.GA.IsZero() && d.EOM.Before(d.GA) {
		r.Errorf(path+".eom", "is before ga")
	}
	if d.EOL.IsZero() {
		return
	}
	if !d.EOM.IsZero() && d.EOL.Before(d.EOM) {
		r.Errorf(path+".eol", "is before eom")
	} else if !d.GA.IsZero() && d.EOL.Before(d.GA) {
		r.Errorf(path+".eol", "is before ga")
	}
}

// checkKubernetes reports a range that is half written, whose ends are not
// minors like 1.35, or whose min is after its max.
func checkKubernetes(r *config.Report, path string, k lines.KubernetesRange) {
	if k.Min == "" && k.Max == "" {
		return
	}
	ok := true
	for _, end := range []struct{ name, value string }{{"min", k.Min}, {"max", k.Max}} {
		switch {
		case end.value == "":
			r.Errorf(path+"."+end.name, "required when the other end is given")
			ok = false
		case !kubernetesMinor.MatchString(end.value):
			r.Errorf(path+"."+end.name, "%q is not a Kubernetes minor: write it like 1.35", end.value)
			ok = false
		}
	}
	if !ok {
		return
	}
	if lines.Compare(k.Min, k.Max) > 0 {
		r.Errorf(path, "min %s is after max %s", k.Min, k.Max)
	}
}
