package config

import (
	"errors"
	"time"
)

// Dates are the lifecycle dates of a line, as published on the SUSE lifecycle
// page. A zero date is not set. Each phase begins at 00:00 UTC on its date.
type Dates struct {
	GA  time.Time `yaml:"ga"`
	EOM time.Time `yaml:"eom"`
	EOL time.Time `yaml:"eol"`
}

// validate checks that the dates are in order (ga <= eom <= eol) and that a
// later date is not set without the ones before it that it depends on: an eom
// needs a ga. An eol may stand without an eom.
func (d Dates) validate() error {
	if !d.EOM.IsZero() && d.GA.IsZero() {
		return errors.New("dates.eom is set without dates.ga")
	}
	if !d.EOM.IsZero() && d.EOM.Before(d.GA) {
		return errors.New("dates.eom is before dates.ga")
	}
	if !d.EOL.IsZero() {
		if !d.EOM.IsZero() && d.EOL.Before(d.EOM) {
			return errors.New("dates.eol is before dates.eom")
		}
		if !d.GA.IsZero() && d.EOL.Before(d.GA) {
			return errors.New("dates.eol is before dates.ga")
		}
	}
	return nil
}

// PhaseAt is the phase of the line at now. An explicit Phase overrides the
// dates. Otherwise the phase follows the dates: a line with no GA date, or one
// still in the future, is in development.
func (l Line) PhaseAt(now time.Time) string {
	if l.Phase != "" {
		return l.Phase
	}
	now = now.UTC()
	switch {
	case reached(l.Dates.EOL, now):
		return PhaseEOL
	case reached(l.Dates.EOM, now):
		return PhaseEOM
	case reached(l.Dates.GA, now):
		return PhaseGA
	default:
		return PhaseDevelopment
	}
}

// reached reports whether the date is set and now is at or after it.
func reached(date, now time.Time) bool {
	return !date.IsZero() && !now.Before(date)
}
