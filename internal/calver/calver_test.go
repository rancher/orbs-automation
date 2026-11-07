package calver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func at(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func TestLineStageParse(t *testing.T) {
	tests := []struct {
		name string
		want Tag
		ok   bool
	}{
		{"v2.15-20260915T1243Z", Tag{Name: "v2.15-20260915T1243Z", Line: "v2.15", Stage: "release", Time: at(2026, 9, 15, 12, 43)}, true},
		{"v2.16-20261030T1111Z-dev", Tag{Name: "v2.16-20261030T1111Z-dev", Line: "v2.16", Stage: "dev", Time: at(2026, 10, 30, 11, 11)}, true},
		{"v2.9-20250101T0000Z", Tag{Name: "v2.9-20250101T0000Z", Line: "v2.9", Stage: "release", Time: at(2025, 1, 1, 0, 0)}, true},
		{"v2.16", Tag{}, false},
		{"v2.16.1", Tag{}, false},
		{"main-abc1234", Tag{}, false},
		{"2.16-20261030T1111Z", Tag{}, false},
		{"v2.16-20261030", Tag{}, false},
		{"v2.16-20261030T1111", Tag{}, false},
		{"v2.16-20261030T1111Z-rc", Tag{}, false},
		{"v2.16-20261030T1111Z-dev-1", Tag{}, false},
		{"xv2.16-20261030T1111Z", Tag{}, false},
		{"v2.16-20261030T1111Z\n", Tag{}, false},
		{"v2.16-20261330T1111Z", Tag{}, false},
		{"v2.16-20261030T2561Z", Tag{}, false},
		{"v2.16-20260231T1111Z", Tag{}, false},
		{"", Tag{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LineStage{}.Parse(tt.name)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLineStageCompare(t *testing.T) {
	a, _ := LineStage{}.Parse("v2.16-20261030T1111Z-dev")
	b, _ := LineStage{}.Parse("v2.16-20261101T0900Z-dev")
	require.False(t, a.Time.IsZero())

	assert.Negative(t, LineStage{}.Compare(a, b))
	assert.Positive(t, LineStage{}.Compare(b, a))
	assert.Zero(t, LineStage{}.Compare(a, a))
}

// The tags of the Assets repository as they come: both stages and several lines
// mixed, a prod tag with no dev twin, and names that are not releases.
var assetsTags = []string{
	"v2.16-20261030T1111Z-dev",
	"v2.16-20261101T0900Z-dev",
	"v2.16-20261029T0000Z",
	"v2.15-20260915T1243Z",
	"v2.15-20260915T1243Z-dev",
	"v2.14-20260401T1000Z",
	"v2.16",
	"main-abc1234",
}

func namesOf(tags []Tag) []string {
	var names []string
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

func TestSelectIsFilteredByLineAndStageThenNewestFirst(t *testing.T) {
	got := Select(LineStage{}, assetsTags, "v2.16", "dev")

	assert.Equal(t, []string{"v2.16-20261101T0900Z-dev", "v2.16-20261030T1111Z-dev"}, namesOf(got))
}

func TestSelectDoesNotMixTheStages(t *testing.T) {
	got := Select(LineStage{}, assetsTags, "v2.16", "release")

	assert.Equal(t, []string{"v2.16-20261029T0000Z"}, namesOf(got), "the dev tags are newer, and are not this stage")
}

func TestSelectOfALineOrStageWithNoTagsIsEmpty(t *testing.T) {
	assert.Empty(t, Select(LineStage{}, assetsTags, "v2.13", "dev"))
	assert.Empty(t, Select(LineStage{}, assetsTags, "v2.14", "dev"))
	assert.Empty(t, Select(LineStage{}, nil, "v2.16", "dev"))
}

func TestLinesAreTheOnesTagsCarryNewestFirst(t *testing.T) {
	assert.Equal(t, []string{"v2.16", "v2.15", "v2.14"}, Lines(LineStage{}, assetsTags))
}

func TestLinesSortByNumberAndNotAsText(t *testing.T) {
	got := Lines(LineStage{}, []string{"v2.9-20250101T0000Z", "v2.10-20250601T0000Z", "v2.9-20250201T0000Z"})
	assert.Equal(t, []string{"v2.10", "v2.9"}, got)
}

func TestLinesOfNoReleasesIsEmpty(t *testing.T) {
	assert.Empty(t, Lines(LineStage{}, []string{"v2.16", "main"}))
	assert.Empty(t, Lines(LineStage{}, nil))
}

func TestRegistry(t *testing.T) {
	s, err := Builtin().Get("calverLineStage")
	require.NoError(t, err)
	assert.Equal(t, "calverLineStage", s.Name())

	_, err = Builtin().Get("semver")
	require.Error(t, err)
	assert.ErrorContains(t, err, "calverLineStage", "the error lists what exists")

	assert.Equal(t, []string{"calverLineStage"}, Builtin().Names())
	assert.Panics(t, func() {
		r := Builtin()
		r.Register(LineStage{})
	}, "two schemes claiming one name is a programming error")
}
