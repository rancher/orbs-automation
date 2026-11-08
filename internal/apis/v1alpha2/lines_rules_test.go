package v1alpha2

import (
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func linesDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n" + spec
}

func loadLines(t *testing.T, spec string) (*RancherLines, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, linesDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*RancherLines), rep
}

func TestValidRancherLinesHaveNoFindings(t *testing.T) {
	for name, spec := range map[string]string{
		"a line with nothing else":     "    - id: v2.16\n",
		"a line with a range":          "    - id: v2.16\n      kubernetes: {min: \"1.35\", max: \"1.37\"}\n",
		"a line with all its dates":    "    - id: v2.15\n      dates: {ga: 2026-08-27, eom: 2027-02-27, eol: 2028-02-27}\n",
		"dates that are the same day":  "    - id: v2.15\n      dates: {ga: 2026-08-27, eom: 2026-08-27}\n",
		"an eol with no eom":           "    - id: v2.15\n      dates: {ga: 2026-08-27, eol: 2028-02-27}\n",
		"a phase":                      "    - id: v2.15\n      phase: eom\n",
		"two lines":                    "    - id: v2.16\n    - id: v2.15\n",
		"a range with equal ends":      "    - id: v2.16\n      kubernetes: {min: \"1.35\", max: \"1.35\"}\n",
		"a range that crosses a major": "    - id: v2.16\n      kubernetes: {min: \"1.9\", max: \"1.10\"}\n",
	} {
		t.Run(name, func(t *testing.T) {
			l, rep := loadLines(t, spec)

			assert.Empty(t, rep.Findings)
			assert.NotNil(t, l)
		})
	}
}

func TestRancherLinesRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"no id", "    - phase: ga\n", "spec.lines[0].id", "required"},
		{"an id that is not a line", "    - id: 2.16\n", "spec.lines[0].id", "v2.16"},
		{"a line listed twice", "    - id: v2.16\n    - id: v2.15\n    - id: v2.16\n", "spec.lines[2].id", "twice"},
		{"a phase that does not exist", "    - id: v2.16\n      phase: beta\n", "spec.lines[0].phase", "development, ga, eom or eol"},
		{"an eom with no ga", "    - id: v2.16\n      dates: {eom: 2027-02-27}\n", "spec.lines[0].dates.eom", "ga"},
		{"an eom before the ga", "    - id: v2.16\n      dates: {ga: 2027-02-27, eom: 2026-08-27}\n", "spec.lines[0].dates.eom", "before"},
		{"an eol before the eom", "    - id: v2.16\n      dates: {ga: 2026-01-01, eom: 2027-02-27, eol: 2026-08-27}\n", "spec.lines[0].dates.eol", "before"},
		{"an eol before the ga", "    - id: v2.16\n      dates: {ga: 2027-01-01, eol: 2026-08-27}\n", "spec.lines[0].dates.eol", "before"},
		{"a kubernetes min that is not a minor", "    - id: v2.16\n      kubernetes: {min: v1.35, max: \"1.37\"}\n", "spec.lines[0].kubernetes.min", "not a Kubernetes minor"},
		{"a kubernetes max that is not a minor", "    - id: v2.16\n      kubernetes: {min: \"1.35\", max: latest}\n", "spec.lines[0].kubernetes.max", "not a Kubernetes minor"},
		{"a kubernetes min after the max", "    - id: v2.16\n      kubernetes: {min: \"1.37\", max: \"1.35\"}\n", "spec.lines[0].kubernetes", "after"},
		{"a kubernetes min with no max", "    - id: v2.16\n      kubernetes: {min: \"1.35\"}\n", "spec.lines[0].kubernetes.max", "required"},
		{"a kubernetes max with no min", "    - id: v2.16\n      kubernetes: {max: \"1.35\"}\n", "spec.lines[0].kubernetes.min", "required"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, rep := loadLines(t, tt.spec)

			assert.Nil(t, l, "lines that break a rule are dropped")
			assert.True(t, hasError(rep, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
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
