package lines

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidIsALineAsItIsWrittenInConfig(t *testing.T) {
	for id, want := range map[string]bool{
		"v2.16": true, "v2.9": true, "v10.0": true,
		"2.16": false, "V2.16": false, "v2": false, "v2.16.1": false, "v2.x": false, "": false, "latest": false, " v2.16": false,
	} {
		assert.Equal(t, want, Valid(id), "%q", id)
	}
}

func TestANumberTooBigToReadIsNotALine(t *testing.T) {
	for _, id := range []string{"v2.99999999999999999999", "v99999999999999999999.1"} {
		assert.False(t, Valid(id), id)
		_, ok := Parse(id)
		assert.False(t, ok, id)
		assert.True(t, (Range{Since: id}).Contains("v2.1"), "an end that is not a line is open: "+id)
	}
}

func TestNormalizeWritesTheOneWayLinesAreCompared(t *testing.T) {
	for in, want := range map[string]string{"v2.16": "v2.16", "2.16": "v2.16", "V2.16": "v2.16", "": "v"} {
		assert.Equal(t, want, Normalize(in), "%q", in)
	}
}

func TestParseReadsALineOrAKubernetesMinor(t *testing.T) {
	for in, want := range map[string]Number{"v2.16": {2, 16}, "2.16": {2, 16}, "V2.9": {2, 9}, "1.35": {1, 35}} {
		got, ok := Parse(in)

		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "latest", "v2", "v2.16.1", "v2.x", "vv2.16", "-2.16"} {
		_, ok := Parse(in)

		assert.False(t, ok, "%q", in)
	}
}

func TestNumberBefore(t *testing.T) {
	assert.True(t, Number{2, 9}.Before(Number{2, 16}), "the minor is a number, not text")
	assert.True(t, Number{1, 99}.Before(Number{2, 0}))
	assert.False(t, Number{2, 16}.Before(Number{2, 16}))
	assert.False(t, Number{3, 0}.Before(Number{2, 99}))
}

func TestCompareOrdersLinesByTheirNumbers(t *testing.T) {
	assert.Negative(t, Compare("v2.9", "v2.16"))
	assert.Positive(t, Compare("v2.16", "v2.9"))
	assert.Zero(t, Compare("v2.16", "2.16"), "with or without the v")
	assert.Negative(t, Compare("v1.99", "v2.0"))
	assert.Negative(t, Compare("v2.16", "latest"), "text that is not a line is after every line")
	assert.Positive(t, Compare("latest", "v2.16"))
	assert.Negative(t, Compare("alpha", "beta"), "and two such texts are ordered as text")
}

func TestARangeContainsALine(t *testing.T) {
	tests := []struct {
		name  string
		rng   Range
		line  string
		count bool
	}{
		{"the zero range holds every line", Range{}, "v2.16", true},
		{"since holds its own line", Range{Since: "v2.15"}, "v2.15", true},
		{"since holds a later line", Range{Since: "v2.15"}, "v2.16", true},
		{"since does not hold an earlier line", Range{Since: "v2.15"}, "v2.14", false},
		{"until holds its own line", Range{Until: "v2.15"}, "v2.15", true},
		{"until holds an earlier line", Range{Until: "v2.15"}, "v2.9", true},
		{"until does not hold a later line", Range{Until: "v2.15"}, "v2.16", false},
		{"both ends", Range{Since: "v2.14", Until: "v2.16"}, "v2.15", true},
		{"the minor is a number, not text", Range{Since: "v2.9"}, "v2.10", true},
		{"the major comes first", Range{Since: "v2.9"}, "v3.0", true},
		{"a line without its v", Range{Since: "v2.15"}, "2.16", true},
		{"a line in capitals", Range{Since: "v2.15"}, "V2.14", false},
		{"a line that is not a line", Range{Since: "v2.15"}, "latest", true},
		{"an end that is not a line is open", Range{Since: "soon"}, "v2.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.count, tt.rng.Contains(tt.line))
		})
	}
}

func TestRangesOverlap(t *testing.T) {
	tests := []struct {
		name string
		a, b Range
		want bool
	}{
		{"two zero ranges", Range{}, Range{}, true},
		{"a range and the zero range", Range{Since: "v2.15"}, Range{}, true},
		{"the same range", Range{Since: "v2.14", Until: "v2.15"}, Range{Since: "v2.14", Until: "v2.15"}, true},
		{"sharing one line", Range{Until: "v2.14"}, Range{Since: "v2.14"}, true},
		{"one after the other", Range{Until: "v2.14"}, Range{Since: "v2.15"}, false},
		{"the other order", Range{Since: "v2.15"}, Range{Until: "v2.14"}, false},
		{"one inside the other", Range{Since: "v2.12", Until: "v2.16"}, Range{Since: "v2.14", Until: "v2.15"}, true},
		{"both open at the top", Range{Since: "v2.14"}, Range{Since: "v2.15"}, true},
		{"the minor is a number, not text", Range{Until: "v2.9"}, Range{Since: "v2.10"}, false},
		{"an end that is not a line is open", Range{Until: "soon"}, Range{Since: "v2.15"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Overlaps(tt.b))
			assert.Equal(t, tt.want, tt.b.Overlaps(tt.a), "it does not depend on the order")
		})
	}
}

func TestALinePhaseFollowsItsDatesAndTheTime(t *testing.T) {
	d := func(day string) time.Time {
		v, err := time.Parse("2006-01-02", day)
		require.NoError(t, err)
		return v
	}
	dated := Line{ID: "v2.14", Dates: Dates{GA: d("2026-04-30"), EOM: d("2026-10-30"), EOL: d("2027-10-10")}}
	tests := []struct {
		name string
		line Line
		now  time.Time
		want string
	}{
		{"no dates", Line{ID: "v2.16"}, d("2026-11-07"), PhaseDevelopment},
		{"before the ga", dated, d("2026-04-29"), PhaseDevelopment},
		{"on the ga date", dated, d("2026-04-30"), PhaseGA},
		{"between ga and eom", dated, d("2026-10-29"), PhaseGA},
		{"on the eom date", dated, d("2026-10-30"), PhaseEOM},
		{"on the eol date", dated, d("2027-10-10"), PhaseEOL},
		{"after the eol date", dated, d("2028-01-01"), PhaseEOL},
		{"a ga with no eom or eol", Line{ID: "v2.15", Dates: Dates{GA: d("2026-08-27")}}, d("2026-11-07"), PhaseGA},
		{"a phase over the dates", Line{ID: "v2.14", Phase: PhaseEOL, Dates: dated.Dates}, d("2026-05-01"), PhaseEOL},
		{"a time that is not in UTC", dated, time.Date(2026, 4, 30, 1, 0, 0, 0, time.FixedZone("x", 2*3600)), PhaseDevelopment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.line.PhaseAt(tt.now))
		})
	}
}
