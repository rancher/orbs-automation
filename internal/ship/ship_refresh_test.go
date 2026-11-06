package ship

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scratchV215 = botV215 + ".refresh"

// refreshing is a run that has already opened the PR once, so the branch is on
// the remote and the fake GitHub reports its PR as open, and then asks to refresh.
func refreshing(t *testing.T) (*realWorkspace, string, *Executor, *fakeRemote, *log, Input) {
	t.Helper()
	ws, clone, ex, remote, lg := publishing(t)
	ex.Now = func() time.Time { return time.Date(2026, 11, 6, 15, 4, 0, 0, time.UTC) }
	first, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)
	require.NoError(t, err)
	require.Equal(t, ResultOpened, onlyResult(t, first).Status)
	open := existing(104, "danpock", "v0.5.3")
	preview := first.Targets[0].Preview
	open.Title, open.Body = preview.Title, preview.Body                           // as the PR was made
	open.HeadOwner, open.HeadRef, open.Base = "rancher", botV215, "release/v2.15" // pushed to the upstream
	remote.open = pulls.Open{PRs: []pulls.PR{open}}
	lg.calls = nil
	in := realRun()
	in.Refresh = true
	return ws, clone, ex, remote, lg, in
}

// withGenerate makes the method run the script after the edit, so a fresh build
// has a commit that the first one did not.
func withGenerate(ws *realWorkspace) {
	ws.links[0].Method.PostEdit = []config.Step{{Run: config.Command{"./scripts/gen.sh"}}}
}

func TestRefreshOfABranchThatIsIdenticalToAFreshBuildChangesNothing(t *testing.T) {
	ws, clone, ex, remote, lg, in := refreshing(t)
	localBefore := runGit(t, clone, "rev-parse", botV215)
	remoteBefore := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Contains(t, res.Note, "identical")
	for _, write := range []string{"push", "update", "edit comment", "comment #104", "close"} {
		assert.False(t, lg.has(write), "nothing is written for an identical branch: %s", write)
	}
	assert.Equal(t, localBefore, runGit(t, clone, "rev-parse", botV215))
	assert.Equal(t, remoteBefore, runGit(t, ws.up, "rev-parse", botV215))
	assert.False(t, branchExistsIn(t, clone, scratchV215), "the scratch branch is gone")
	_ = remote
}

// targetMoves puts a commit on the target that the PR's branch does not have, as
// a busy base does, and has GitHub say the PR is behind by it.
func targetMoves(t *testing.T, ws *realWorkspace, remote *fakeRemote) {
	t.Helper()
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.15")
	put(t, ws.up, "unrelated.txt", "the target moved on")
	runGit(t, ws.up, "add", ".")
	runGit(t, ws.up, "commit", "--quiet", "-m", "unrelated")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	remote.behind = 1
}

func TestRefreshOfABranchThatIsBehindIsReplacedEvenWhenTheChangeIsIdentical(t *testing.T) {
	ws, clone, ex, remote, lg, in := refreshing(t)
	targetMoves(t, ws, remote)
	oldTip := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	require.Equal(t, ResultRefreshed, res.Status, "behind is out of date, so it is rebuilt on the current head")
	assert.Equal(t, 1, res.BehindBy)
	newTip := runGit(t, ws.up, "rev-parse", botV215)
	assert.NotEqual(t, oldTip, newTip)
	assert.Equal(t, "1", runGit(t, ws.up, "rev-list", "--count", "release/v2.15.."+botV215), "on top of the moved target")
	assert.False(t, branchExistsIn(t, clone, scratchV215))
	require.True(t, lg.has("comment #104 "))
	assert.Contains(t, lg.calls[lg.index("comment #104 ")], "was 1 behind")
}

func TestRefreshOfABranchThatIsBehindIsOnlyReportedWithoutTheFlag(t *testing.T) {
	ws, _, ex, remote, lg, _ := refreshing(t)
	targetMoves(t, ws, remote)
	oldTip := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Equal(t, 1, res.BehindBy)
	assert.Equal(t, oldTip, runGit(t, ws.up, "rev-parse", botV215))
	assert.False(t, lg.has("update"))
}

const staleBody = "## Summary\nan older text\n\n"

func TestRefreshUpdatesAStaleDescriptionAndKeepsOtherHiddenBlocksWhenTheBranchIsFine(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	pr := &remote.open.PRs[0]
	fresh := pr.Body
	pr.Title = "an older title"
	pr.Body = staleBody + ownMarker(pr.Body) + "\n\n" + trackerBlock + "\n"
	oldTip := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status, "the branch is as it should be")
	assert.Contains(t, res.Note, "description")
	require.True(t, lg.has("update #104"), "the title and body are brought up to date")
	assert.Equal(t, "[v2.15] Update scc-operator to v0.5.3", remote.updatedTitle)
	assert.Equal(t, mergeBody(pr.Body, fresh), remote.updatedBody)
	assert.Contains(t, remote.updatedBody, trackerBlock, "what another part of gero put there stays")
	assert.NotContains(t, remote.updatedBody, "an older text")
	assert.Equal(t, oldTip, runGit(t, ws.up, "rev-parse", botV215), "and the branch is not pushed")
	assert.False(t, lg.has("push"))
	require.True(t, lg.has("comment #104 "), "the update is recorded")
	assert.Contains(t, lg.calls[lg.index("comment #104 ")], "Updated the description")
}

func TestRefreshUpdatesATitleThatDiffersAloneAndNeverTouchesABodyThatDoesNot(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.open.PRs[0].Title = "an older title"

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	require.True(t, lg.has("update #104"))
	assert.Equal(t, remote.open.PRs[0].Body, remote.updatedBody, "the body was already right")
}

func TestRefreshLeavesTheDescriptionAloneWhenOnlyTheLineEndingsDiffer(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.open.PRs[0].Body = strings.ReplaceAll(remote.open.PRs[0].Body, "\n", "\r\n") + "\r\n"

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.False(t, lg.has("update"), "GitHub stores the line endings it likes")
}

func TestRefreshADryRunSaysItWouldUpdateTheDescriptionAndWritesNothing(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.open.PRs[0].Body = staleBody + ownMarker(remote.open.PRs[0].Body) + "\n"
	in.DryRun = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Contains(t, res.Note, "would update the description")
	for _, write := range []string{"push", "update", "edit comment", "comment #104", "close"} {
		assert.False(t, lg.has(write), "a dry run does not %s", write)
	}
}

func TestRefreshOfAReplacedBranchMergesTheBodyToo(t *testing.T) {
	ws, _, ex, remote, _, in := refreshing(t)
	pr := &remote.open.PRs[0]
	pr.Body += "\n" + trackerBlock + "\n"
	withGenerate(ws)

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Contains(t, remote.updatedBody, trackerBlock, "a replaced branch does not erase the tracker's block")
	assert.Contains(t, remote.updatedBody, "Ran `./scripts/gen.sh`", "and the new text is there")
}

// ownMarker is the marker block of a body made by gero ship.
func ownMarker(body string) string {
	for _, b := range geroBlocks(body) {
		if b.kind == "ship" {
			return b.text
		}
	}
	return ""
}

func TestRefreshOfABranchThatDiffersReplacesItAndRecordsIt(t *testing.T) {
	ws, clone, ex, remote, lg, in := refreshing(t)
	oldTip := runGit(t, ws.up, "rev-parse", botV215)
	withGenerate(ws)
	rec := &recording{}

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, rec)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultRefreshed, res.Status)
	require.NotNil(t, res.PR)
	assert.Equal(t, 104, res.PR.Number, "the same PR")
	assert.Equal(t, "upstream", res.Pushed)
	require.Len(t, res.Commits, 2, "the edit and what the script made")
	assert.NotEmpty(t, res.Place, "where the rebuild was made, as for any build")
	newTip := runGit(t, ws.up, "rev-parse", botV215)
	assert.NotEqual(t, oldTip, newTip)
	assert.Equal(t, res.Tip(), newTip, "the remote's branch is the rebuilt one")
	assert.Equal(t, newTip, runGit(t, clone, "rev-parse", botV215), "and so is the local one")
	assert.False(t, branchExistsIn(t, clone, scratchV215))
	assert.Equal(t, "2", runGit(t, ws.up, "rev-list", "--count", "release/v2.15.."+botV215))
	assert.True(t, lg.has(`update #104 title="[v2.15] Update scc-operator to v0.5.3"`), "the title and body are refreshed")
	assert.Contains(t, rec.kinds(), EventRefreshed)
	assert.Contains(t, rec.kinds(), EventPushed)
	require.True(t, lg.has("comment #104 "), "recorded in a comment")
	comment := lg.calls[lg.index("comment #104 ")]
	assert.Contains(t, comment, "<!-- gero:refresh v1 -->")
	assert.Contains(t, comment, "Rebuilt on release/v2.15@")
	assert.Contains(t, comment, "2026-11-06")
	_ = remote
}

func TestRefreshReplacesABranchThatIsCheckedOutInTheClone(t *testing.T) {
	ws, clone, ex, _, _, in := refreshing(t)
	runGit(t, clone, "checkout", "--quiet", botV215) // the engineer is on it
	oldTip := runGit(t, ws.up, "rev-parse", botV215)
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	require.Equal(t, ResultRefreshed, res.Status, "error: %s", res.Error)
	newTip := runGit(t, ws.up, "rev-parse", botV215)
	assert.NotEqual(t, oldTip, newTip)
	assert.Equal(t, res.Tip(), newTip, "the remote has the rebuilt commit")
	assert.Equal(t, oldTip, runGit(t, clone, "rev-parse", botV215), "the checked-out branch is not moved under them")
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, res.Warnings[0], "checked out")
	assert.False(t, branchExistsIn(t, clone, scratchV215))
}

func TestRefreshEditsTheOneCommentInPlaceAndDoesNotAddAnother(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.comments = []pulls.Comment{
		{ID: 40, Author: "alice", Body: "a comment of someone else's\n<!-- gero:refresh v1 -->"},
		{ID: 55, Author: "danpock", Body: "Rebuilt by gero ship:\n\n- Rebuilt on release/v2.15@1111111 (2026-11-01 09:00 UTC)\n\n<!-- gero:refresh v1 -->"},
	}
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultRefreshed, onlyResult(t, plan).Status)
	assert.False(t, lg.has("comment #104 "), "no second comment")
	require.True(t, lg.has("edit comment 55 "))
	edited := lg.calls[lg.index("edit comment 55 ")]
	assert.Contains(t, edited, "2026-11-01 09:00 UTC", "the earlier line is kept")
	assert.Contains(t, edited, "2026-11-06 15:04 UTC", "and the new one is added")
	assert.False(t, lg.has("edit comment 40"), "never someone else's comment")
}

func TestRefreshACommentOfSomeoneElsesIsNeverEditedSoANewOneIsMade(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.comments = []pulls.Comment{{ID: 40, Author: "alice", Body: "<!-- gero:refresh v1 -->"}}
	withGenerate(ws)

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	assert.False(t, lg.has("edit comment"))
	assert.True(t, lg.has("comment #104 "))
}

func TestRefreshWillNotReplaceABranchWithACommitThatIsNotOurs(t *testing.T) {
	ws, _, ex, _, lg, in := refreshing(t)
	runGit(t, ws.up, "checkout", "--quiet", botV215)
	put(t, ws.up, "hand-made.txt", "someone's fixup")
	runGit(t, ws.up, "add", ".")
	runGit(t, ws.up, "commit", "--quiet", "--author=Dev <dev@example.com>", "-m", "a fixup by hand")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	withGenerate(ws)
	handTip := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Contains(t, res.Note, "not refreshed")
	assert.Contains(t, res.Note, "trailer")
	assert.Equal(t, handTip, runGit(t, ws.up, "rev-parse", botV215), "their commit is untouched")
	assert.False(t, lg.has("update"))
}

func TestRefreshWillNotTouchABranchInAnotherRepository(t *testing.T) {
	ws, _, ex, remote, lg, in := refreshing(t)
	remote.open.PRs[0].HeadOwner = "someone-else"
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Contains(t, res.Note, "not refreshed")
	assert.Contains(t, res.Note, "someone-else")
	assert.False(t, lg.has("update"))
}

func TestRefreshWillNotTouchABranchWithAnotherName(t *testing.T) {
	ws, _, ex, remote, _, in := refreshing(t)
	remote.open.PRs[0].HeadRef = "dan/scc-operator-v0.5.3-v2.15"
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Contains(t, res.Note, "not refreshed")
	assert.Contains(t, res.Note, "dan/scc-operator-v0.5.3-v2.15")
	assert.Contains(t, res.Note, botV215)
}

func TestRefreshADryRunSaysItWouldAndChangesNothing(t *testing.T) {
	ws, clone, ex, _, lg, in := refreshing(t)
	localBefore := runGit(t, clone, "rev-parse", botV215)
	remoteBefore := runGit(t, ws.up, "rev-parse", botV215)
	withGenerate(ws)
	in.DryRun = true

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultWouldRefresh, res.Status)
	assert.Len(t, res.Commits, 2)
	for _, write := range []string{"push", "update", "edit comment", "comment #104", "close"} {
		assert.False(t, lg.has(write), "a dry run does not %s", write)
	}
	assert.Equal(t, remoteBefore, runGit(t, ws.up, "rev-parse", botV215))
	assert.Equal(t, localBefore, runGit(t, clone, "rev-parse", botV215), "nor does it move the local branch")
	assert.False(t, branchExistsIn(t, clone, scratchV215))
}

func TestRefreshAFailureToUpdateTheBodyOrRecordItIsAWarningAndTheBranchStaysReplaced(t *testing.T) {
	ws, _, ex, remote, _, in := refreshing(t)
	remote.updateErr = errors.New("update refused")
	remote.commentsErr = errors.New("cannot list comments")
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultRefreshed, res.Status)
	require.Len(t, res.Warnings, 2)
	assert.Contains(t, res.Warnings[0], "update refused")
	assert.Contains(t, res.Warnings[1], "cannot list comments")
}

func TestWithoutRefreshAnOpenPRIsNotRebuiltEvenIfItWouldDiffer(t *testing.T) {
	ws, _, ex, _, lg, _ := refreshing(t)
	withGenerate(ws)

	plan, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)

	require.NoError(t, err)
	assert.Equal(t, ResultAlreadyOpen, onlyResult(t, plan).Status)
	assert.False(t, lg.has("push"))
	assert.False(t, lg.has("update"))
}

func TestRefreshIsPartOfWhatTheConfirmationAsksAbout(t *testing.T) {
	ws, _, ex, _, _, in := refreshing(t)
	q := &asked{answer: true}
	ex.Confirm = q.confirm
	withGenerate(ws)

	_, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	require.Len(t, q.got, 1)
	assert.Equal(t, []string{"v2.15"}, q.got[0].Refreshes, "named by line, as the report names it")
	assert.Empty(t, q.got[0].Opens)
}

func TestShipThePlanSaysWhetherRefreshWasAsked(t *testing.T) {
	ws, _, ex, _, _, in := refreshing(t)

	asked, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)
	require.NoError(t, err)
	plain, err := Ship(bg, ws, &fakeProber{}, realRun(), ex, nil)
	require.NoError(t, err)

	assert.True(t, asked.Refresh)
	assert.False(t, plain.Refresh)
}
