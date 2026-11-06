package ship

import (
	"errors"
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

func TestRefreshOfABranchThatIsBehindButTheSameChangeIsIdentical(t *testing.T) {
	ws, clone, ex, remote, _, in := refreshing(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v2.15")
	put(t, ws.up, "unrelated.txt", "the target moved on")
	runGit(t, ws.up, "add", ".")
	runGit(t, ws.up, "commit", "--quiet", "-m", "unrelated")
	runGit(t, ws.up, "checkout", "--quiet", "main")
	remote.behind = 1
	remoteBefore := runGit(t, ws.up, "rev-parse", botV215)

	plan, err := Ship(bg, ws, &fakeProber{}, in, ex, nil)

	require.NoError(t, err)
	res := onlyResult(t, plan)
	assert.Equal(t, ResultAlreadyOpen, res.Status)
	assert.Contains(t, res.Note, "identical", "the same change, though the base moved: not worth a force push")
	assert.Equal(t, 1, res.BehindBy)
	assert.Equal(t, remoteBefore, runGit(t, ws.up, "rev-parse", botV215))
	assert.False(t, branchExistsIn(t, clone, scratchV215))
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
	assert.Equal(t, []string{"release/v2.15"}, q.got[0].Refreshes)
	assert.Empty(t, q.got[0].Opens)
}
