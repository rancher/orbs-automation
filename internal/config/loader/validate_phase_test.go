package loader

import (
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(m time.Month, d int) time.Time {
	return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
}

// datedLines is valid() with the explicit phases replaced by dates: v2.16 has
// none (development) and v2.15 went GA on 2026-08-27.
func datedLines() config.Config {
	c := valid()
	c.Lines.Lines = []config.Line{
		{ID: "v2.16"},
		{ID: "v2.15", Dates: config.Dates{GA: day(time.August, 27)}},
	}
	return c
}

func TestValidateAtAcceptsLinesWithoutExplicitPhase(t *testing.T) {
	assert.NoError(t, ValidateAt(datedLines(), day(time.November, 4)))
}

func TestValidateAtRejectsDatesOutOfOrder(t *testing.T) {
	c := datedLines()
	c.Lines.Lines[1].Dates.EOM = day(time.January, 1)

	err := ValidateAt(c, day(time.November, 4))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lines[1]")
	assert.Contains(t, err.Error(), "dates.eom is before dates.ga")
}

func TestValidateAtBranchRuleFollowsTheDerivedPhase(t *testing.T) {
	c := datedLines()
	// v2.15 has a branch; v2.16 does not, which is fine while it is in development.
	require.NoError(t, ValidateAt(c, day(time.November, 4)))

	// Once v2.16 goes GA, a line with no branch is a problem.
	c.Lines.Lines[0].Dates.GA = day(time.December, 1)

	assert.NoError(t, ValidateAt(c, day(time.November, 30)))
	err := ValidateAt(c, day(time.December, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2.16")
	assert.Contains(t, err.Error(), "has no branch")
	assert.Contains(t, err.Error(), `"ga"`)
}

func TestValidateRejectsUnknownPhaseOverrideButNotAnEmptyOne(t *testing.T) {
	c := datedLines()
	c.Lines.Lines[0].Phase = "beta"

	assert.ErrorContains(t, ValidateAt(c, day(time.November, 4)), `"beta"`)
}
