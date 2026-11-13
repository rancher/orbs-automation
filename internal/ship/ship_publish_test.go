package ship

import (
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishing is a real run, as an engineer would make it, with the fake GitHub:
// the branch is pushed to the clone's upstream remote, which is a repository on
// disk.
func publishing(t *testing.T) (*realWorkspace, string, *Executor, *fakeRemote, *log) {
	t.Helper()
	ws, clone, ex := newShip(t)
	lg := &log{}
	remote := &fakeRemote{log: lg, labels: []string{"status/auto-created", "team/orbs"}}
	ws.push = PushSettings{Upstream: "upstream"} // no fork, so the upstream is where it goes
	ex.Publish = &Publisher{Git: GitCLI{}, Remote: remote}
	return ws, clone, ex, remote, lg
}

func realRun() Input { return Input{Team: testTeam, Project: "scc-operator", Lines: []string{"v2.15"}} }

// mine and others' PRs for the bump that the fixture ships, v0.5.3 to release/v2.15.
func existing(number int, author, version string) pulls.PR {
	return pr(number, author, Marker{
		Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: "release/v2.15", Version: version,
	})
}

func TestShipPublishesAPlainRunAndClosesMyOlderPR(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(101, "danpock", "v0.5.2")}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number)
	assert.Equal(t, "upstream", res.Pushed)
	assert.Equal(t, []PRRef{{101, "https://github.com/rancher/rancher/pull/101"}}, withoutURL(res.Superseded, "https://github.com/rancher/rancher/pull/101"))
	assert.Equal(t, res.Tip(), runGit(t, ws.up, "rev-parse", botV215), "the branch is on the remote")
	assert.Equal(t, "main", current(t, clone))
	assert.Less(t, lg.index("open rancher/rancher release/v2.15"), lg.index("create "), "the open PRs are looked at before one is made")
	assert.Less(t, lg.index("create "), lg.index("comment #101"))
	assert.Less(t, lg.index("comment #101"), lg.index("close #101"))
}

// withoutURL fills in the url of a reference, which the fake derives from the number.
func withoutURL(refs []PRRef, url string) []PRRef {
	out := make([]PRRef, len(refs))
	for i, r := range refs {
		out[i] = PRRef{Number: r.Number, URL: url}
	}
	return out
}

func TestShipAnAlreadyOpenPRMakesNothingAndClosesTheOlderOnes(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(104, "danpock", "v0.5.3"), existing(101, "danpock", "v0.5.2")}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number)
	assert.False(t, branchExistsIn(t, clone, botV215), "no branch was made")
	assert.False(t, lg.has("create"))
	assert.False(t, lg.has("push"))
	assert.True(t, lg.has("comment #101 Superseded by #104"), "the older one is told the open one supersedes it")
	assert.True(t, lg.has("close #101"))
	assert.Empty(t, res.Commits)
}

func TestShipANewerPROfMineBlocksOnlyThatTarget(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(110, "danpock", "v0.5.4")}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultBlocked, res.Status)
	assert.Contains(t, res.Error, "#110")
	assert.Contains(t, res.Error, "v0.5.4")
	assert.False(t, branchExistsIn(t, clone, botV215))
	assert.False(t, lg.has("create"))
	assert.False(t, lg.has("close"))
}

func TestShipAnotherAuthorsPRIsLeftAloneAndReported(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(120, "rancher-bot[bot]", "v0.5.3"), existing(101, "danpock", "v0.5.2")}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status, "it does not block")
	assert.False(t, lg.has("close #120"))
	assert.False(t, lg.has("comment #120"))
	assert.True(t, lg.has("close #101"))
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "#120")
	assert.Contains(t, res.Warnings[0], "rancher-bot[bot]")
}

func TestShipANoPushRunNeverLooksAtGitHub(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	in := realRun()
	in.NoPush = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultCommitted, onlyResult(t, plan).Status)
	assert.Empty(t, lg.calls, "nothing was asked of GitHub and nothing was pushed")
	assert.True(t, branchExistsIn(t, clone, botV215))
}

func TestShipADryRunLooksButChangesNothingAndSaysWhatItWouldClose(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(101, "danpock", "v0.5.2")}}
	in := realRun()
	in.DryRun = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultWouldCommit, res.Status)
	require.Len(t, res.WouldSupersede, 1)
	assert.Equal(t, 101, res.WouldSupersede[0].Number)
	assert.True(t, lg.has("open rancher/rancher release/v2.15"), "reading is allowed")
	for _, write := range []string{"create", "comment", "close", "add labels", "push"} {
		assert.False(t, lg.has(write), "a dry run does not %s", write)
	}
	assert.False(t, branchExistsIn(t, clone, botV215))
	_, pushed := gitBranchOnRemote(t, ws.up, botV215)
	assert.False(t, pushed)
}

// gitBranchOnRemote reports whether the repository on disk has the branch.
func gitBranchOnRemote(t *testing.T, repo, branch string) (string, bool) {
	t.Helper()
	out := runGit(t, repo, "branch", "--list", branch)
	return out, out != ""
}

func TestShipNotKnowingWhoIsRunningStopsBeforeAnyWork(t *testing.T) {
	ws, clone, ex, remote, _ := publishing(t)
	remote.meErr = errors.New("bad credentials")

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.ErrorIs(t, err, ErrNoRemote)
	assert.Contains(t, err.Error(), "bad credentials")
	assert.Equal(t, StatusReady, plan.Status, "the plan is still returned")
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipAFailedLookupOfOpenPRsFailsThatTargetBeforeAnyWork(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.openErr = errors.New("rate limited")

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, "rate limited")
	assert.False(t, branchExistsIn(t, clone, botV215))
	assert.False(t, lg.has("push"))
}

func TestShipAListingThatWasCutShortIsAWarning(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{Truncated: true}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status)
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "newest")
}

func TestShipEmitsTheStepsOfPublishingInOrder(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{existing(101, "danpock", "v0.5.2")}}
	rec := &recording{}

	_, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, rec)

	require.NoError(t, err)
	assert.Equal(t, []EventKind{
		EventResolved, EventTargetChecked, EventTargetsChecked, EventImagesChecked,
		EventDecided, EventTargetStarted, EventCommit, EventPushed, EventPROpened, EventSuperseded, EventTargetDone, EventDone,
	}, rec.kinds())
	decided := rec.first(t, EventDecided)
	require.NotNil(t, decided.Decision)
	assert.Equal(t, PublishCreate, decided.Decision.Action)
}

func TestShipPublishesTheBranchAnEarlierNoPushRunLeftWithoutBuildingItAgain(t *testing.T) {
	ws, clone, ex, _, lg := publishing(t)
	noPush := realRun()
	noPush.NoPush = true
	first, err := Ship(bg, ws, &fakeProber{}, noPush, ex, nil)
	require.NoError(t, err)
	made := onlyResult(t, first)
	require.Equal(t, ResultCommitted, made.Status)
	tip := runGit(t, clone, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status, "the branch from the first run is what was published")
	assert.Equal(t, made.Commits, res.Commits, "the same commits, not built again")
	assert.Equal(t, tip, runGit(t, clone, "rev-parse", botV215))
	assert.Equal(t, tip, runGit(t, ws.up, "rev-parse", botV215), "and it is on the remote")
	assert.True(t, lg.has("create"))
	assert.Equal(t, "main", current(t, clone))
}

func TestShipARerunOfANoPushRunReportsTheExistingBranch(t *testing.T) {
	ws, clone, ex, _, _ := publishing(t)
	in := realRun()
	in.NoPush = true
	first, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)
	require.NoError(t, err)
	tip := runGit(t, clone, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultCommitted, res.Status)
	assert.Equal(t, Existing, res.Place)
	assert.Equal(t, onlyResult(t, first).Commits, res.Commits)
	assert.Equal(t, tip, runGit(t, clone, "rev-parse", botV215))
}

// mine is an existing PR of the actor's for the fixture's bump, with its head.
func mine() pulls.PR {
	p := existing(104, "danpock", "v0.5.3")
	p.HeadOwner, p.HeadRef, p.Base = "danpock", botV215, "release/v2.15"
	return p
}

func TestShipAnAlreadyOpenPRSaysHowFarBehindItIs(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{mine()}}
	remote.behind = 7

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Equal(t, 7, res.BehindBy)
	assert.True(t, lg.has("behind release/v2.15 danpock:"+botV215), "asked about the PR's own branch in the fork")
	assert.False(t, branchExistsIn(t, clone, botV215), "and nothing was built: a rebuild needs --refresh")
	assert.False(t, lg.has("push"))
}

func TestShipAnAlreadyOpenPRThatIsCurrentIsNotBehind(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{mine()}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Zero(t, onlyResult(t, plan).BehindBy)
}

func TestShipNotBeingAbleToTellHowFarBehindIsAWarning(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{mine()}}
	remote.behindErr = errors.New("rate limited")

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "behind")
	assert.Contains(t, res.Warnings[0], "rate limited")
}

func TestShipADryRunAlsoSaysHowFarBehind(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.open = pulls.Open{PRs: []pulls.PR{mine()}}
	remote.behind = 3
	in := realRun()
	in.DryRun = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, 3, onlyResult(t, plan).BehindBy)
}

func TestShipAPRWithNoKnownHeadIsNotAskedAbout(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	gone := existing(104, "danpock", "v0.5.3") // the fork it was from is deleted: no head owner
	remote.open = pulls.Open{PRs: []pulls.PR{gone}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultAlreadyOpen, onlyResult(t, plan).Status)
	assert.False(t, lg.has("behind"))
}

func TestShipFindsMyPRByItsHeadWhenTheListingDoesNotReachIt(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	inTheList := mine()
	inTheList.HeadOwner = "rancher"           // pushed to the upstream
	remote.open = pulls.Open{Truncated: true} // the newest 100 did not include it
	remote.byHead = map[string][]pulls.PR{"rancher:" + botV215: {inTheList}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status, "found by the exact head, whatever the listing reached")
	assert.True(t, lg.has("open head rancher/rancher rancher:"+botV215))
	assert.False(t, lg.has("create"))
	assert.False(t, branchExistsIn(t, clone, botV215))
}

func TestShipLooksForTheHeadInTheForkWhenPushingThere(t *testing.T) {
	ws, clone, ex, remote, lg := publishing(t)
	withFork(t, ws, clone)
	remote.byHead = map[string][]pulls.PR{"danpock:" + botV215: {mine()}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultAlreadyOpen, onlyResult(t, plan).Status)
	assert.True(t, lg.has("open head rancher/rancher danpock:"+botV215))
}

func TestShipAPRInBothTheListingAndTheHeadLookupIsOnePR(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	p := mine()
	p.HeadOwner = "rancher"
	remote.open = pulls.Open{PRs: []pulls.PR{p}}
	remote.byHead = map[string][]pulls.PR{"rancher:" + botV215: {p}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.False(t, lg.has("close #104"), "it is not its own duplicate")
	assert.Empty(t, res.Superseded)
}

func TestShipAFailedLookupByHeadIsAWarningAndTheListingStillDecides(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	remote.headErr = errors.New("rate limited")

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status)
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "rate limited")
	assert.Contains(t, res.Warnings[0], botV215)
}

func TestShipANoPushRunDoesNotLookForAHead(t *testing.T) {
	ws, _, ex, _, lg := publishing(t)
	in := realRun()
	in.NoPush = true

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.False(t, lg.has("open head"))
}

// earlierAttempt puts a branch of the name on the remote, as an earlier attempt
// left it: made on the target, by the actor, with the trailer, so the commits are
// ours but are not the ones this run builds.
func earlierAttempt(t *testing.T, ws *realWorkspace, author string, trailer bool) string {
	t.Helper()
	runGit(t, ws.up, "checkout", "--quiet", "-b", botV215, "release/v2.15")
	put(t, ws.up, "earlier.txt", "an earlier attempt")
	runGit(t, ws.up, "add", ".")
	message := "Update scc-operator\n\nAn earlier attempt"
	if trailer {
		message += "\n\nAutomation: gero ship"
	}
	runGit(t, ws.up, "commit", "--quiet", "--author="+author, "-m", message)
	tip := runGit(t, ws.up, "rev-parse", "HEAD")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	return tip
}

func TestShipAnEarlierAttemptsBranchOnTheRemoteStopsAPlainRunWithAWayForward(t *testing.T) {
	ws, _, ex, _, lg := publishing(t)
	theirs := earlierAttempt(t, ws, "Dev <dev@example.com>", true)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, "--refresh", "it says how to go on")
	assert.Contains(t, res.Error, botV215)
	assert.Equal(t, theirs, runGit(t, ws.up, "rev-parse", botV215), "the remote's branch is untouched")
	assert.False(t, lg.has("create"), "no PR was opened for a branch that was not pushed")
}

func TestShipRefreshReplacesAnEarlierAttemptsBranchAndOpensThePR(t *testing.T) {
	ws, _, ex, _, lg := publishing(t)
	theirs := earlierAttempt(t, ws, "Dev <dev@example.com>", true)
	in := realRun()
	in.Refresh = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status)
	assert.NotEqual(t, theirs, runGit(t, ws.up, "rev-parse", botV215))
	assert.Equal(t, res.Tip(), runGit(t, ws.up, "rev-parse", botV215), "the branch is the one this run built")
	assert.True(t, lg.has("create"))
}

func TestShipWillNotReplaceABranchWithCommitsThatAreNotOurs(t *testing.T) {
	ws, _, ex, _, lg := publishing(t)
	theirs := earlierAttempt(t, ws, "Someone <someone@example.com>", true)
	in := realRun()
	in.Refresh = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultFailed, res.Status)
	assert.Contains(t, res.Error, "someone@example.com", "whose commit it is")
	assert.Equal(t, theirs, runGit(t, ws.up, "rev-parse", botV215), "even --refresh does not replace it")
	assert.False(t, lg.has("create"))
}

func TestShipABranchAlreadyOnTheRemoteAtTheSameCommitJustGetsItsPR(t *testing.T) {
	ws, _, ex, remote, lg := publishing(t)
	remote.createErr = errors.New("A pull request already exists, or GitHub is down")
	first, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)
	require.NoError(t, err)
	require.Equal(t, ResultFailed, onlyResult(t, first).Status, "pushed, but the PR was not opened")
	pushed := runGit(t, ws.up, "rev-parse", botV215)
	remote.createErr = nil
	lg.calls = nil

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultOpened, onlyResult(t, plan).Status)
	assert.Equal(t, pushed, runGit(t, ws.up, "rev-parse", botV215), "nothing new to push")
	assert.True(t, lg.has("create"))
}

func TestShipSaysWhenAPRsBranchDoesNotMatchItsMarker(t *testing.T) {
	ws, _, ex, remote, _ := publishing(t)
	renamed := existing(101, "danpock", "v0.5.2")
	renamed.HeadRef = "dan/my-own-name"
	remote.open = pulls.Open{PRs: []pulls.PR{renamed}}

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultOpened, res.Status, "the marker is the identity, so the run goes on")
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "#101")
	assert.Contains(t, res.Warnings[0], "dan/my-own-name")
}
