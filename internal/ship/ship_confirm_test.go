package ship

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asked records the confirmations it was given, and answers them.
type asked struct {
	got    []Confirmation
	answer bool
	err    error
}

func (a *asked) confirm(_ context.Context, c Confirmation) (bool, error) {
	a.got = append(a.got, c)
	return a.answer, a.err
}

// everyBranch ships to both release branches of the fixture.
func everyBranch() Input { return Input{Project: "scc-operator"} }

// onBase is an existing PR of the actor's for the bump to a base.
func onBase(base string, number int, author, version string) pulls.PR {
	return pr(number, author, Marker{Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: base, Version: version})
}

func TestShipAsksOnceBeforeAnythingIsWrittenAndSaysWhatWillBeDone(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.openBy = map[string]pulls.Open{
		"release/v2.15": {PRs: []pulls.PR{onBase("release/v2.15", 101, "danpock", "v0.5.2")}},
		"release/v2.14": {PRs: []pulls.PR{onBase("release/v2.14", 104, "danpock", "v0.5.3")}},
	}
	q := &asked{answer: true}
	ex.Confirm = func(ctx context.Context, c Confirmation) (bool, error) {
		// Nothing has been written when the question is asked: no branch, no push.
		assert.False(t, branchExistsIn(t, clone, botV215), "no branch yet")
		assert.False(t, lg.has("push"))
		assert.False(t, lg.has("create"))
		assert.False(t, lg.has("close"))
		return q.confirm(ctx, c)
	}

	_, err := Ship(bg, ws, &fakeProber{}, everyBranch(), ex, nil)

	require.NoError(t, err)
	require.Len(t, q.got, 1, "once, for the whole run")
	c := q.got[0]
	assert.Equal(t, "rancher/rancher", c.Consumer)
	assert.Equal(t, "v0.5.3", c.Release)
	assert.Equal(t, "upstream", c.Push.Remote)
	assert.Equal(t, []string{"v2.15"}, c.Opens, "named by line, as the report names it")
	assert.Equal(t, []string{"v2.14"}, c.Already)
	assert.Empty(t, c.Blocked)
	require.Len(t, c.Closes, 1)
	assert.Equal(t, 101, c.Closes[0].Number)
}

func TestShipDecliningWritesNothingAndReturnsThePlan(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	q := &asked{answer: false}
	ex.Confirm = q.confirm

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.ErrorIs(t, err, ErrDeclined)
	assert.Equal(t, StatusReady, plan.Status, "the plan is still returned")
	assert.Nil(t, plan.Targets[0].Result, "no work was done")
	assert.False(t, branchExistsIn(t, clone, botV215), "not even a local branch")
	assert.Equal(t, "main", current(t, clone))
	assert.False(t, lg.has("push"))
	assert.False(t, lg.has("create"))
	assert.False(t, lg.has("close"))
}

func TestShipAnErrorFromTheQuestionStopsBeforeAnyWork(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	boom := errors.New("there is no terminal to ask on: pass --yes")
	q := &asked{err: boom}
	ex.Confirm = q.confirm

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.ErrorIs(t, err, boom)
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipIsNotAskedWhenNothingWouldBeWritten(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(104, "danpock", "v0.5.3")}} // already open, nothing older
	q := &asked{answer: true}
	ex.Confirm = q.confirm

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Empty(t, q.got)
	assert.Equal(t, ResultAlreadyOpen, onlyResult(t, plan).Status)
}

func TestShipIsNotAskedWhenEverythingIsBlocked(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(110, "danpock", "v0.5.4")}}
	q := &asked{answer: true}
	ex.Confirm = q.confirm

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Empty(t, q.got)
}

func TestShipAClosingOfOlderPRsAloneIsAWriteThatIsAskedAbout(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(104, "danpock", "v0.5.3"), existing(101, "danpock", "v0.5.2")}}
	q := &asked{answer: false}
	ex.Confirm = q.confirm

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.ErrorIs(t, err, ErrDeclined)
	require.Len(t, q.got, 1)
	assert.Empty(t, q.got[0].Opens)
	assert.Equal(t, []string{"v2.15"}, q.got[0].Already)
	assert.Len(t, q.got[0].Closes, 1)
	assert.False(t, lg.has("close"), "declined, so the older PR is still open")
}

func TestShipIsNotAskedForADryRunOrANoPushRun(t *testing.T) {
	for _, in := range []Input{
		{Project: "scc-operator", Lines: []string{"v2.15"}, DryRun: true},
		{Project: "scc-operator", Lines: []string{"v2.15"}, NoPush: true},
	} {
		ws, _, ex, _, _ := publishing(t)
		q := &asked{answer: false}
		ex.Confirm = q.confirm

		_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

		require.NoError(t, err)
		assert.Empty(t, q.got, "dry run %v, no push %v", in.DryRun, in.NoPush)
	}
}

func TestShipWithNoConfirmGoesOn(t *testing.T) {
	ws, _, ex, _, _ := publishing(t)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultOpened, onlyResult(t, plan).Status)
}

func TestShipEveryTargetIsDecidedBeforeTheFirstOneStarts(t *testing.T) {
	ws, _, ex, _, _ := publishing(t)
	ex.Confirm = (&asked{answer: true}).confirm
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, everyBranch(), ex, rec)

	require.NoError(t, err)
	kinds := rec.kinds()
	lastDecided, firstStarted := -1, len(kinds)
	for i, k := range kinds {
		if k == EventDecided {
			lastDecided = i
		}
		if k == EventTargetStarted && i < firstStarted {
			firstStarted = i
		}
	}
	assert.Less(t, lastDecided, firstStarted, "all the lookups come first, so the question is accurate: %v", kinds)
	assert.Len(t, rec.of(EventDecided), 2)
}

func TestShipTheQuestionSaysWhenARefreshMayReplaceABranchOnTheRemote(t *testing.T) {
	ws, _, ex, _, _ := publishing(t)
	q := &asked{answer: false}
	ex.Confirm = q.confirm
	in := realRun()
	in.Refresh = true

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.ErrorIs(t, err, ErrDeclined)
	require.Len(t, q.got, 1)
	assert.Equal(t, []string{"v2.15"}, q.got[0].Opens)
	assert.True(t, q.got[0].MayReplace)
}

func TestShipTheQuestionDoesNotSayItWithoutRefresh(t *testing.T) {
	ws, _, ex, _, _ := publishing(t)
	q := &asked{answer: false}
	ex.Confirm = q.confirm

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.ErrorIs(t, err, ErrDeclined)
	require.Len(t, q.got, 1)
	assert.False(t, q.got[0].MayReplace)
}
