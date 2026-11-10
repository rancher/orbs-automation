package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unowned is the fixture's producer made like a project whose releases another
// team ships: gero reads it and never writes for it.
func unowned(ws *fakeWorkspace, team string) *fakeWorkspace {
	ws.producer.ManagedBy = team
	return ws
}

func TestCheckRefusesAProducerThatIsNotOwned(t *testing.T) {
	_, err := Check(ctx, unowned(fixture(t), "release-team"), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned)
	assert.ErrorContains(t, err, "scc-operator")
	assert.ErrorContains(t, err, "release-team", "it names who ships it")
	assert.ErrorContains(t, err, "not owned by "+testTeam, "and the team gero is acting for")
}

func TestCheckRefusesAProducerThatNamesNoTeamWithoutNamingOne(t *testing.T) {
	_, err := Check(ctx, unowned(fixture(t), ""), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned)
	assert.NotContains(t, err.Error(), "ships its releases", "no team is named to ship it")
}

func TestCheckRefusesASharedProducer(t *testing.T) {
	_, err := Check(ctx, unowned(fixture(t), "shared"), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned, "no team owns a shared project")
}

func TestCheckRefusesWhenNoTeamIsActive(t *testing.T) {
	_, err := Check(ctx, fixture(t), &fakeProber{}, Input{Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned)
	assert.ErrorContains(t, err, "the active team")
}

func TestHoldingsAreStillReadForAProducerThatIsNotOwned(t *testing.T) {
	got, err := holdings(t, unowned(loopFixture(t), "release-team"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotEmpty(t, got[0].Targets, "observing is reading, which the refusal to ship does not touch")
}
