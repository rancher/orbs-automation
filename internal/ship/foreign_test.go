package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foreign is the fixture's producer made like another team's project, shipped
// into a consumer managed by consumerTeam, whose edit has steps from the producer.
func foreign(t *testing.T, consumerTeam string) *fakeWorkspace {
	t.Helper()
	ws := unowned(loopFixture(t), "release-team")
	l := stepsLink()
	l.Consumer.ManagedBy = consumerTeam
	ws.links = []link{l}
	return ws
}

func TestCheckShipsAnotherTeamsReleaseWhenToNamesAConsumerTheTeamOwns(t *testing.T) {
	got, err := Check(ctx, foreign(t, testTeam), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", got.Producer)
	assert.Equal(t, "rancher/rancher", got.Consumer)
	assert.Equal(t, StatusReady, got.Status)
}

func TestCheckShipsAnotherTeamsReleaseIntoASharedConsumer(t *testing.T) {
	got, err := Check(ctx, foreign(t, "shared"), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

	require.NoError(t, err)
	assert.Equal(t, "rancher/rancher", got.Consumer)
}

func TestCheckRefusesAnotherTeamsReleaseIntoAConsumerNoOneOfTheTeamsOwns(t *testing.T) {
	for name, team := range map[string]string{"another team's": "frameworks-team", "unspecified": ""} {
		t.Run(name, func(t *testing.T) {
			_, err := Check(ctx, foreign(t, team), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

			require.ErrorIs(t, err, ErrNotOwned)
			assert.ErrorContains(t, err, "rancher", "it names the consumer")
			assert.ErrorContains(t, err, "release-team", "and who manages the producer")
		})
	}
}

func TestCheckStillRefusesAnotherTeamsReleaseWithNoTo(t *testing.T) {
	_, err := Check(ctx, foreign(t, testTeam), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator"})

	require.ErrorIs(t, err, ErrNotOwned, "a bare ship never takes another team's project")
}

func TestCheckSkipsTheStepsFromAnotherTeamsProducerAndSaysSo(t *testing.T) {
	got, err := Check(ctx, foreign(t, testTeam), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

	require.NoError(t, err)
	require.Len(t, got.Notes, 1)
	assert.Contains(t, got.Notes[0], "release-team", "it says whose project the steps are of")
	assert.Contains(t, got.Notes[0], "may be incomplete")
	assert.Contains(t, got.Notes[0], "pre-edit")
	previews := 0
	for _, tp := range got.Targets {
		if tp.Preview == nil {
			continue // up to date: nothing to write, so no PR text
		}
		previews++
		assert.NotContains(t, tp.Preview.Body, "- Ran `pre-edit`", "a step from the producer is not run")
		assert.Contains(t, tp.Preview.Body, "- Skipped `pre-edit`", "the PR says it was skipped")
		assert.Contains(t, tp.Preview.Body, "- Ran `pre-method`", "a step from the consumer's clone still runs")
		for _, st := range tp.Preview.Steps {
			assert.NotEqual(t, "pre-edit", st.Run, "no commit is previewed for a skipped step")
		}
	}
	assert.NotZero(t, previews, "a target needs a PR")
}

func TestCheckRunsTheStepsFromAProducerTheTeamOwns(t *testing.T) {
	ws := loopFixture(t)
	ws.links = []link{stepsLink()}

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

	require.NoError(t, err)
	assert.Empty(t, got.Notes)
	previews := 0
	for _, tp := range got.Targets {
		if tp.Preview == nil {
			continue
		}
		previews++
		assert.Contains(t, tp.Preview.Body, "- Ran `pre-edit`")
		assert.NotContains(t, tp.Preview.Body, "Skipped")
	}
	assert.NotZero(t, previews, "a target needs a PR")
}

func TestAnOwnedProducerShipsToAnyConsumerWithTo(t *testing.T) {
	ws := loopFixture(t)
	l := stepsLink()
	l.Consumer.ManagedBy = "frameworks-team"
	ws.links = []link{l}

	got, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher"})

	require.NoError(t, err, "the team's own producer goes to any consumer that holds it")
	assert.Equal(t, "rancher/rancher", got.Consumer)
}

// fromTheProducer makes the fixture's edit run a step from the producer's repo,
// which the real step runner refuses until it is built.
func fromTheProducer(ws *realWorkspace) {
	ws.links[0].Consumer.ManagedBy = testTeam
	ws.links[0].Post = []stepSpec{{Run: "true", From: "producer"}}
}

func TestShipRunsToTheEndForAnotherTeamsProducerBecauseItSkipsTheStep(t *testing.T) {
	ws, _, ex := newShip(t)
	fromTheProducer(ws)
	unowned(ws.fakeWorkspace, "release-team")
	in := shipIn("v2.15")
	in.To = "rancher"

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultCommitted, onlyResult(t, plan).Status, "the step from the producer was never run")
	require.Len(t, plan.Notes, 1)
	assert.Contains(t, plan.Notes[0], "may be incomplete")
}

func TestShipStopsAtAStepFromAProducerTheTeamOwnsThatIsNotBuilt(t *testing.T) {
	ws, _, ex := newShip(t)
	fromTheProducer(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status, "what is refused for the team's own producer is only skipped for another team's")
	assert.Contains(t, res.Error, "not supported yet")
}
