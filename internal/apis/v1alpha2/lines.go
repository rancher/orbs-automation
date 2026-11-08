package v1alpha2

import "time"

// KindRancherLines is the kind of the one document that lists the Rancher lines.
const KindRancherLines = "RancherLines"

// RancherLines is the set of Rancher lines and facts about each. It is public
// product data, with no branches: each project maps its own.
type RancherLines struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       RancherLinesSpec `yaml:"spec"`
}

// RancherLinesSpec is the fields of RancherLines.
type RancherLinesSpec struct {
	Lines []Line `yaml:"lines"`
}

// Line is one Rancher line.
type Line struct {
	ID         string          `yaml:"id"`
	Phase      string          `yaml:"phase"` // optional override: development, ga, eom or eol
	Dates      Dates           `yaml:"dates"`
	Kubernetes KubernetesRange `yaml:"kubernetes"`
}

// Dates are the lifecycle dates of a line, as published on the SUSE lifecycle
// page. A zero date is not set.
type Dates struct {
	GA  time.Time `yaml:"ga"`
	EOM time.Time `yaml:"eom"`
	EOL time.Time `yaml:"eol"`
}

// KubernetesRange is the supported Kubernetes minor range of a line.
type KubernetesRange struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

// PhaseAt is the phase of the line at now: the phase it is forced into, else the
// one its dates give. Each phase begins at 00:00 UTC on its date, and a line with
// no ga date, or one still to come, is in development.
func (l Line) PhaseAt(now time.Time) string {
	if l.Phase != "" {
		return l.Phase
	}
	now = now.UTC()
	reached := func(date time.Time) bool { return !date.IsZero() && !now.Before(date) }
	switch {
	case reached(l.Dates.EOL):
		return PhaseEOL
	case reached(l.Dates.EOM):
		return PhaseEOM
	case reached(l.Dates.GA):
		return PhaseGA
	}
	return PhaseDevelopment
}
