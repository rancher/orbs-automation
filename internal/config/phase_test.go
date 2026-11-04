package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestPhaseAt(t *testing.T) {
	dated := Line{ID: "v2.14", Dates: Dates{
		GA:  day(2026, time.April, 30),
		EOM: day(2026, time.October, 30),
		EOL: day(2027, time.October, 10),
	}}
	tests := []struct {
		name string
		line Line
		now  time.Time
		want string
	}{
		{"no dates is development", Line{ID: "v2.16"}, day(2026, time.November, 4), PhaseDevelopment},
		{"before GA is development", dated, day(2026, time.April, 29), PhaseDevelopment},
		{"on the GA date is ga", dated, day(2026, time.April, 30), PhaseGA},
		{"between GA and EOM is ga", dated, day(2026, time.July, 1), PhaseGA},
		{"day before EOM is ga", dated, day(2026, time.October, 29), PhaseGA},
		{"on the EOM date is eom", dated, day(2026, time.October, 30), PhaseEOM},
		{"day before EOL is eom", dated, day(2027, time.October, 9), PhaseEOM},
		{"on the EOL date is eol", dated, day(2027, time.October, 10), PhaseEOL},
		{"long after EOL is eol", dated, day(2030, time.January, 1), PhaseEOL},
		{
			"only GA set stays ga",
			Line{ID: "v2.15", Dates: Dates{GA: day(2026, time.August, 27)}},
			day(2031, time.January, 1),
			PhaseGA,
		},
		{
			"override wins over dates",
			Line{ID: "v2.14", Phase: PhaseEOL, Dates: dated.Dates},
			day(2026, time.July, 1),
			PhaseEOL,
		},
		{
			"override on a line with no dates",
			Line{ID: "v2.16", Phase: PhaseGA},
			day(2026, time.November, 4),
			PhaseGA,
		},
		{
			"a clock in another zone is read as UTC",
			dated,
			time.Date(2026, time.April, 29, 20, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
			PhaseGA, // 2026-04-30 01:00 UTC
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.line.PhaseAt(tt.now))
		})
	}
}

func TestDatesValidate(t *testing.T) {
	tests := []struct {
		name  string
		dates Dates
		want  string // empty means valid
	}{
		{"none", Dates{}, ""},
		{"all in order", Dates{GA: day(2026, 1, 1), EOM: day(2026, 7, 1), EOL: day(2027, 1, 1)}, ""},
		{"same day is allowed", Dates{GA: day(2026, 1, 1), EOM: day(2026, 1, 1)}, ""},
		{"eom before ga", Dates{GA: day(2026, 7, 1), EOM: day(2026, 1, 1)}, "eom"},
		{"eol before eom", Dates{GA: day(2026, 1, 1), EOM: day(2026, 7, 1), EOL: day(2026, 3, 1)}, "eol"},
		{"eol before ga with no eom", Dates{GA: day(2026, 7, 1), EOL: day(2026, 1, 1)}, "eol"},
		{"eom without ga", Dates{EOM: day(2026, 7, 1)}, "ga"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dates.validate()
			if tt.want == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
