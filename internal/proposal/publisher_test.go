package proposal

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal/proposaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pushOnly struct {
	Git
	log *proposaltest.Log
	err error
}

// Fetch says the branch is not on the remote, so the push is the ordinary one.
func (g pushOnly) Fetch(context.Context, string, string, string) (string, error) {
	return "", errors.New("couldn't find remote ref")
}

func (g pushOnly) Push(_ context.Context, dir, remote, branch string) error {
	g.log.Add("push %s %s %s", dir, remote, branch)
	return g.err
}

type publishCase struct {
	lg     *proposaltest.Log
	remote *proposaltest.FakeRemote
	pub    Publisher
	tp     Subject
	res    *TargetResult
}

func newPublishCase() *publishCase {
	lg := &proposaltest.Log{}
	remote := &proposaltest.FakeRemote{Log: lg, Labels: []string{"status/auto-created", "team/orbs", "origin/bot"}}
	return &publishCase{
		lg: lg, remote: remote,
		pub: Publisher{
			Git: pushOnly{log: lg}, Remote: remote, Clone: "/src/rancher",
			Push:      PushTarget{Role: RoleFork, Remote: "origin"},
			HeadOwner: "danpock",
		},
		tp: Subject{
			Branch: "release/v2.15",
			Preview: &Preview{
				Branch: "bot/scc-operator-v0.5.4-v2.15", Title: "[v2.15] Update scc-operator to v0.5.4", Body: "the body",
				Labels: []string{"status/auto-created", "team/orbs"},
			},
		},
		res: &TargetResult{Status: ResultCommitted, Branch: "bot/scc-operator-v0.5.4-v2.15"},
	}
}

func (c *publishCase) run(dec Decision) {
	c.pub.Publish(context.Background(), "rancher/rancher", c.tp, markerFor("v0.5.4"), dec, c.res, nil)
}

func old(numbers ...int) Decision {
	d := Decision{Action: PublishCreate}
	for _, n := range numbers {
		d.Supersede = append(d.Supersede, pulls.PR{Number: n, URL: fmt.Sprintf("https://github.com/rancher/rancher/pull/%d", n)})
	}
	return d
}

func TestPublishPushesOpensLabelsThenClosesTheOlderOnesInThatOrder(t *testing.T) {
	c := newPublishCase()

	c.run(old(101, 90))

	assert.Equal(t, []string{
		"push /src/rancher origin bot/scc-operator-v0.5.4-v2.15",
		`create rancher/rancher head=danpock:bot/scc-operator-v0.5.4-v2.15 base=release/v2.15 title="[v2.15] Update scc-operator to v0.5.4"`,
		"labels",
		"add labels #104 status/auto-created,team/orbs",
		"comment #101 Superseded by #104 (rancher/scc-operator v0.5.4).",
		"close #101",
		"comment #90 Superseded by #104 (rancher/scc-operator v0.5.4).",
		"close #90",
	}, c.lg.Calls)
	assert.Equal(t, ResultOpened, c.res.Status)
	assert.Equal(t, &PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"}, c.res.PR)
	assert.Equal(t, "origin", c.res.Pushed)
	assert.Equal(t, []PRRef{{Number: 101, URL: "https://github.com/rancher/rancher/pull/101"}, {Number: 90, URL: "https://github.com/rancher/rancher/pull/90"}}, c.res.Superseded)
	assert.Empty(t, c.res.Warnings)
}

func TestPublishTheHeadIsTheBareBranchForTheUpstream(t *testing.T) {
	c := newPublishCase()
	c.pub.Push = PushTarget{Role: RoleUpstream, Remote: "upstream"}

	c.run(old())

	require.GreaterOrEqual(t, len(c.lg.Calls), 2)
	assert.Contains(t, c.lg.Calls[1], "head=bot/scc-operator-v0.5.4-v2.15 ", "no owner for a branch in the repository itself")
	assert.Equal(t, "upstream", c.res.Pushed)
}

func TestPublishWithNothingToCloseMakesNoComments(t *testing.T) {
	c := newPublishCase()

	c.run(old())

	assert.False(t, c.lg.Has("comment"))
	assert.False(t, c.lg.Has("close"))
	assert.Empty(t, c.res.Superseded)
}

func TestPublishOnlyLabelsThatExistAreAppliedAndTheOthersAreWarnings(t *testing.T) {
	c := newPublishCase()
	c.remote.Labels = []string{"status/auto-created"}

	c.run(old())

	assert.Contains(t, c.lg.Calls, "add labels #104 status/auto-created")
	assert.Equal(t, ResultOpened, c.res.Status, "a missing label never fails the PR")
	require.Len(t, c.res.Warnings, 1)
	assert.Contains(t, c.res.Warnings[0], "team/orbs")
	assert.Contains(t, c.res.Warnings[0], "not in rancher/rancher")
}

func TestPublishLabelsMatchWithoutRegardToCase(t *testing.T) {
	c := newPublishCase()
	c.remote.Labels = []string{"Status/Auto-Created", "TEAM/orbs"}

	c.run(old())

	assert.Contains(t, c.lg.Calls, "add labels #104 Status/Auto-Created,TEAM/orbs", "the repository's own spelling is what is sent")
	assert.Empty(t, c.res.Warnings)
}

func TestPublishNoLabelsConfiguredReadsNoLabels(t *testing.T) {
	c := newPublishCase()
	c.tp.Preview.Labels = nil

	c.run(old())

	assert.False(t, c.lg.Has("labels"))
	assert.False(t, c.lg.Has("add labels"))
}

func TestPublishBotLabelsOnlyWhenTheBotIsTheActor(t *testing.T) {
	human := newPublishCase()
	human.pub.BotLabels = []string{"origin/bot"}
	human.run(old())
	assert.Contains(t, human.lg.Calls, "add labels #104 status/auto-created,team/orbs", "an engineer's PR is not labelled as a bot's")

	bot := newPublishCase()
	bot.pub.IsBot, bot.pub.BotLabels = true, []string{"origin/bot"}
	bot.run(old())
	assert.Contains(t, bot.lg.Calls, "add labels #104 status/auto-created,team/orbs,origin/bot")
}

func TestPublishALabelFailureIsAWarningAndTheOlderPRsAreStillClosed(t *testing.T) {
	c := newPublishCase()
	c.remote.AddErr = errors.New("label deleted meanwhile")

	c.run(old(101))

	assert.Equal(t, ResultOpened, c.res.Status)
	require.Len(t, c.res.Warnings, 1)
	assert.Contains(t, c.res.Warnings[0], "label deleted meanwhile")
	assert.True(t, c.lg.Has("close #101"))
}

func TestPublishNotBeingAbleToReadTheLabelsIsAWarning(t *testing.T) {
	c := newPublishCase()
	c.remote.LabelsErr = errors.New("rate limited")

	c.run(old())

	assert.Equal(t, ResultOpened, c.res.Status)
	require.Len(t, c.res.Warnings, 1)
	assert.Contains(t, c.res.Warnings[0], "rate limited")
	assert.False(t, c.lg.Has("add labels"), "no labels are sent that may not exist")
}

func TestPublishAFailedPushFailsTheTargetAndOpensNothing(t *testing.T) {
	c := newPublishCase()
	c.pub.Git = pushOnly{log: c.lg, err: errors.New("the remote rejected the push")}

	c.run(old(101))

	assert.Equal(t, ResultFailed, c.res.Status)
	assert.Contains(t, c.res.Error, "push")
	assert.Contains(t, c.res.Error, "rejected")
	assert.False(t, c.lg.Has("create"))
	assert.False(t, c.lg.Has("comment"))
	assert.Nil(t, c.res.PR)
}

func TestPublishAFailedPRLeavesTheOlderOnesAloneAndSaysTheBranchIsPushed(t *testing.T) {
	c := newPublishCase()
	c.remote.CreateErr = errors.New("A pull request already exists")

	c.run(old(101))

	assert.Equal(t, ResultFailed, c.res.Status)
	assert.Contains(t, c.res.Error, "already exists")
	assert.Contains(t, c.res.Error, "pushed to origin", "so the person knows what is on the remote")
	assert.Equal(t, "origin", c.res.Pushed)
	assert.False(t, c.lg.Has("comment"), "only comment once the new PR exists")
	assert.False(t, c.lg.Has("close"))
}

func TestPublishFailingToCloseAnOlderPRIsAWarningAndTheOthersAreStillClosed(t *testing.T) {
	c := newPublishCase()
	c.remote.CloseOn = map[int]error{101: errors.New("403 forbidden")}

	c.run(old(101, 90))

	assert.Equal(t, ResultOpened, c.res.Status)
	require.Len(t, c.res.Warnings, 1)
	assert.Contains(t, c.res.Warnings[0], "#101")
	assert.Contains(t, c.res.Warnings[0], "403 forbidden")
	assert.True(t, c.lg.Has("close #90"))
	assert.Equal(t, []PRRef{{Number: 90, URL: "https://github.com/rancher/rancher/pull/90"}}, c.res.Superseded, "only what was closed")
}

func TestPublishFailingToCommentLeavesThatPROpen(t *testing.T) {
	c := newPublishCase()
	c.remote.CommentOn = map[int]error{101: errors.New("locked")}

	c.run(old(101))

	assert.False(t, c.lg.Has("close #101"), "it was not told it is superseded, so it is not closed")
	require.Len(t, c.res.Warnings, 1)
	assert.Contains(t, c.res.Warnings[0], "#101")
	assert.Empty(t, c.res.Superseded)
}

func (c *publishCase) runWith(dec Decision, sink Sink) {
	c.pub.Publish(context.Background(), "rancher/rancher", c.tp, markerFor("v0.5.4"), dec, c.res, sink)
}

func TestPublishEmitsAnEventForEachStepAsItIsDone(t *testing.T) {
	c := newPublishCase()
	rec := &recording{}

	c.runWith(old(101, 90), rec)

	assert.Equal(t, []EventKind{EventPushed, EventPROpened, EventSuperseded, EventSuperseded}, rec.kinds())
	pushed := rec.first(t, EventPushed)
	assert.Equal(t, "origin", pushed.Remote)
	assert.Equal(t, "release/v2.15", pushed.Branch)
	opened := rec.first(t, EventPROpened)
	require.NotNil(t, opened.PR)
	assert.Equal(t, 104, opened.PR.Number)
	closed := rec.of(EventSuperseded)
	require.Len(t, closed, 2)
	require.NotNil(t, closed[0].PR)
	assert.Equal(t, 101, closed[0].PR.Number)
	assert.Equal(t, 90, closed[1].PR.Number)
}

func TestPublishAFailedPushEmitsNoEvent(t *testing.T) {
	c := newPublishCase()
	c.pub.Git = pushOnly{log: c.lg, err: errors.New("no")}
	rec := &recording{}

	c.runWith(old(101), rec)

	assert.Empty(t, rec.events)
}

func TestPublishAnOlderPRThatWasNotClosedEmitsNoEvent(t *testing.T) {
	c := newPublishCase()
	c.remote.CloseOn = map[int]error{101: errors.New("403")}
	rec := &recording{}

	c.runWith(old(101), rec)

	assert.Equal(t, []EventKind{EventPushed, EventPROpened}, rec.kinds())
}
