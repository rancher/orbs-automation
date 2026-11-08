// Package lines is what every version of the config API says of a Rancher line: its
// id, a range of lines, and the lifecycle facts of a line (its dates, its phase
// and the Kubernetes versions it supports). The values have no apiVersion of
// their own, so a version of the API uses them as they are.
package lines

import "time"

// The phases a line may be in.
const (
	PhaseDevelopment = "development"
	PhaseGA          = "ga"
	PhaseEOM         = "eom"
	PhaseEOL         = "eol"
)

// Number is a line id, or a Kubernetes minor, as its major and minor.
type Number [2]int

// Before reports whether n is an earlier line than o.
func (n Number) Before(Number) bool { return false }

// Valid reports whether id is a line as it is written in config: v2.16.
func Valid(string) bool { return false }

// Normalize writes a line the one way lines are compared: lowercase, with its v.
// It does not check that the text is a line.
func Normalize(string) string { return "" }

// Parse reads a line written with or without its v, in any case, such as 2.16 or
// V2.16, or a Kubernetes minor such as 1.35. It is false for anything else.
func Parse(string) (Number, bool) { return Number{}, false }

// Compare orders two lines by their numbers, so v2.9 is before v2.16: negative when
// a is the earlier. Text that is not a line is after every line, and two such texts
// are ordered as text.
func Compare(_, _ string) int { return 0 }

// Range limits something to the Rancher lines from Since to Until, either of
// which may be left out. The zero Range is every line.
type Range struct {
	Since string `yaml:"since"` // a line id: from this line on
	Until string `yaml:"until"` // a line id: up to and including this line
}

// Contains reports whether a line is in the range, its ends included. A line may
// be written with or without its v, in any case. A line that does not read as a
// Rancher line is counted as in the range, as an end that does not read is as open.
func (r Range) Contains(string) bool { return false }

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
func (l Line) PhaseAt(time.Time) string { return "" }
