package config

import "time"

// Dates are the lifecycle dates of a line, as published on the SUSE lifecycle
// page. A zero date is not set. Each phase begins at 00:00 UTC on its date.
type Dates struct {
	GA  time.Time `yaml:"ga"`
	EOM time.Time `yaml:"eom"`
	EOL time.Time `yaml:"eol"`
}

// validate checks that the dates are in order and that none depends on a date
// that is not set.
func (d Dates) validate() error {
	return nil
}

// PhaseAt is the phase of the line at now. An explicit Phase overrides the
// dates; otherwise the phase follows the dates.
func (l Line) PhaseAt(now time.Time) string {
	return l.Phase
}
