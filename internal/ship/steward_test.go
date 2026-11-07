package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unowned is the fixture's producer made like a project whose releases another
// team ships: gero reads it and never writes for it.
func unowned(ws *fakeWorkspace, team string) *fakeWorkspace {
	ws.producer.Repo.Owned, ws.producer.Repo.ManagedBy = false, team
	return ws
}

func TestCheckRefusesAProducerThatIsNotOwned(t *testing.T) {
	_, err := Check(ctx, unowned(fixture(), "release-team"), &fakeProber{}, Input{Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned)
	assert.ErrorContains(t, err, "scc-operator")
	assert.ErrorContains(t, err, "release-team", "it names who ships it")
}

func TestCheckRefusesAnUnownedProducerThatNamesNoTeamWithoutNamingOne(t *testing.T) {
	_, err := Check(ctx, unowned(fixture(), ""), &fakeProber{}, Input{Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned)
	assert.NotContains(t, err.Error(), "orbs", "an unowned project is never assumed to be orbs's")
}

func TestHoldingsAreStillReadForAProducerThatIsNotOwned(t *testing.T) {
	got, err := holdings(t, unowned(loopFixture(), "release-team"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotEmpty(t, got[0].Targets, "observing is reading, which the refusal to ship does not touch")
}
