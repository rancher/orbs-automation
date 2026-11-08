// Package lines is what every version of the config API says of a Rancher line: its
// id, a range of lines, and the lifecycle facts of a line (its dates, its phase
// and the Kubernetes versions it supports). The values have no apiVersion of
// their own, so a version of the API uses them as they are.
package lines

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
func (n Number) Before(o Number) bool { return n[0] < o[0] || (n[0] == o[0] && n[1] < o[1]) }

// lineID is what a Rancher line looks like in config.
var lineID = regexp.MustCompile(`^v[0-9]+\.[0-9]+$`)

// Valid reports whether id is a line as it is written in config: v2.16.
func Valid(id string) bool { return lineID.MatchString(id) }

// Normalize writes a line the one way lines are compared: lowercase, with its v.
// It does not check that the text is a line.
func Normalize(id string) string {
	id = strings.ToLower(id)
	if !strings.HasPrefix(id, "v") {
		id = "v" + id
	}
	return id
}

// Parse reads a line written with or without its v, in any case, such as 2.16 or
// V2.16, or a Kubernetes minor such as 1.35. It is false for anything else.
func Parse(text string) (Number, bool) {
	id := Normalize(text)
	if !Valid(id) {
		return Number{}, false
	}
	major, minor, _ := strings.Cut(strings.TrimPrefix(id, "v"), ".")
	var n Number
	n[0], _ = strconv.Atoi(major)
	n[1], _ = strconv.Atoi(minor)
	return n, true
}

// Compare orders two lines by their numbers, so v2.9 is before v2.16: negative when
// a is the earlier. Text that is not a line is after every line, and two such texts
// are ordered as text.
func Compare(a, b string) int {
	na, aok := Parse(a)
	nb, bok := Parse(b)
	switch {
	case aok && bok:
		switch {
		case na.Before(nb):
			return -1
		case nb.Before(na):
			return 1
		}
		return 0
	case aok:
		return -1
	case bok:
		return 1
	}
	return strings.Compare(a, b)
}

// Range limits something to the Rancher lines from Since to Until, either of
// which may be left out. The zero Range is every line.
type Range struct {
	Since string `yaml:"since"` // a line id: from this line on
	Until string `yaml:"until"` // a line id: up to and including this line
}

// Contains reports whether a line is in the range, its ends included. A line may
// be written with or without its v, in any case. A line that does not read as a
// Rancher line is counted as in the range, as an end that does not read is as open.
func (r Range) Contains(line string) bool {
	n, ok := Parse(line)
	if !ok {
		return true
	}
	if since, ok := Parse(r.Since); ok && n.Before(since) {
		return false
	}
	if until, ok := Parse(r.Until); ok && until.Before(n) {
		return false
	}
	return true
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
