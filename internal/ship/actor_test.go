package ship

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dan = Identity{Name: "Dan Pock", Email: "dan@example.com"}

// identifying is an Identify that returns id, and says which clone it was asked
// about.
func identifying(id Identity, asked *string) func(context.Context, string) (Identity, error) {
	return func(_ context.Context, clone string) (Identity, error) {
		if asked != nil {
			*asked = clone
		}
		return id, nil
	}
}

func TestShipSaysWhoItActsAsBeforeAnyWorkIsDone(t *testing.T) {
	ws, clone, ex := newShip(t)
	var asked string
	ex.Identify = identifying(dan, &asked)
	rec := &recording{}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, rec)

	require.NoError(t, err)
	require.NotNil(t, plan.Actor)
	assert.Equal(t, Actor{Identity: dan}, *plan.Actor, "not signing, and no GitHub login as nothing is opened")
	assert.Equal(t, clone, asked, "asked about the clone the commits are made in, whose own config may differ")
	kinds := rec.kinds()
	actor, started := indexOf(kinds, EventActor), indexOf(kinds, EventTargetStarted)
	require.GreaterOrEqual(t, actor, 0, "the event is emitted")
	assert.Less(t, actor, started)
	require.NotNil(t, rec.first(t, EventActor).Actor)
	assert.Equal(t, dan, rec.first(t, EventActor).Actor.Identity)
}

func TestShipSaysWhetherTheCommitsAreSignedAndWhichAccountOpensThePRs(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	brokenSigning(t, clone) // the commit will not be made, which does not change what was said
	ex.Identify, ex.Sign = identifying(dan, nil), true
	in := realRun()

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	require.NotNil(t, plan.Actor)
	assert.True(t, plan.Actor.Signing)
	assert.Equal(t, "danpock", plan.Actor.Login, "the PR author is the owner of the token")
}

func TestShipFailsBeforeWritingAnythingWhenItCannotSayWhoItIs(t *testing.T) {
	ws, clone, ex := newShip(t)
	ex.Identify = func(context.Context, string) (Identity, error) {
		return Identity{}, errors.New("git has no user.email")
	}

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "commit identity")
	assert.ErrorContains(t, err, "user.email")
	assert.False(t, branchExistsIn(t, clone, botV215), "nothing was made")
	for _, tp := range plan.Targets {
		assert.Nil(t, tp.Result)
	}
}

func TestShipDoesNotAskWhoItIsWhenThereIsNothingToDo(t *testing.T) {
	ws, _, ex := newShip(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.15")
	put(t, ws.up, "build.yaml", newImage)
	runGit(t, ws.up, "commit", "--quiet", "-am", "already there")
	asked := false
	ex.Identify = func(context.Context, string) (Identity, error) { asked = true; return dan, nil }

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, plan.Status)
	assert.False(t, asked)
	assert.Nil(t, plan.Actor)
}

func TestShipInCICommitsAsTheBotNotAsWhateverGitHas(t *testing.T) {
	ws, clone, ex := newShip(t)
	runGit(t, clone, "config", "user.email", "someone@example.com")
	bot := Identity{Name: "rancher-bot[bot]", Email: "123+rancher-bot[bot]@users.noreply.github.com"}
	ex.Identify = identifying(bot, nil)
	in := shipIn("v2.15")
	in.CI = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	require.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	assert.Equal(t, bot.Email, runGit(t, clone, "log", "-1", "--format=%ae", botV215))
	assert.Equal(t, bot.Name, runGit(t, clone, "log", "-1", "--format=%an", botV215))
	assert.Equal(t, bot.Email, runGit(t, clone, "log", "-1", "--format=%ce", botV215), "the committer too")
}

func TestShipOnAWorkstationLeavesTheAuthorToGit(t *testing.T) {
	ws, clone, ex := newShip(t)
	runGit(t, clone, "config", "user.email", "mine@example.com")
	ex.Identify = identifying(Identity{Name: "Someone Else", Email: "other@example.com"}, nil)

	plan, err := Ship(bg, ws, &fakeProber{}, shipIn("v2.15"), ex, nil)

	require.NoError(t, err)
	require.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	assert.Equal(t, "mine@example.com", runGit(t, clone, "log", "-1", "--format=%ae", botV215),
		"what is shown is what git reads in the clone, so there is nothing to override")
}

func indexOf(kinds []EventKind, kind EventKind) int {
	for i, k := range kinds {
		if k == kind {
			return i
		}
	}
	return -1
}
