package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// datedLines is valid() with the explicit phases replaced by dates: v2.16 has
// none (development) and v2.15 went GA on 2026-08-27.
func datedLines() Config {
	c := valid()
	c.Lines.Lines = []Line{
		{ID: "v2.16"},
		{ID: "v2.15", Dates: Dates{GA: day(2026, time.August, 27)}},
	}
	return c
}

func TestValidateAtAcceptsLinesWithoutExplicitPhase(t *testing.T) {
	assert.NoError(t, datedLines().ValidateAt(day(2026, time.November, 4)))
}

func TestValidateAtRejectsDatesOutOfOrder(t *testing.T) {
	c := datedLines()
	c.Lines.Lines[1].Dates.EOM = day(2026, time.January, 1)

	err := c.ValidateAt(day(2026, time.November, 4))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lines[1]")
	assert.Contains(t, err.Error(), "dates.eom is before dates.ga")
}

func TestValidateAtBranchRuleFollowsTheDerivedPhase(t *testing.T) {
	c := datedLines()
	// v2.15 has a branch; v2.16 does not, which is fine while it is in development.
	require.NoError(t, c.ValidateAt(day(2026, time.November, 4)))

	// Once v2.16 goes GA, a line with no branch is a problem.
	c.Lines.Lines[0].Dates.GA = day(2026, time.December, 1)

	assert.NoError(t, c.ValidateAt(day(2026, time.November, 30)))
	err := c.ValidateAt(day(2026, time.December, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2.16")
	assert.Contains(t, err.Error(), "has no branch")
	assert.Contains(t, err.Error(), `"ga"`)
}

func TestValidateRejectsUnknownPhaseOverrideButNotAnEmptyOne(t *testing.T) {
	c := datedLines()
	c.Lines.Lines[0].Phase = "beta"

	assert.ErrorContains(t, c.ValidateAt(day(2026, time.November, 4)), `"beta"`)
}
