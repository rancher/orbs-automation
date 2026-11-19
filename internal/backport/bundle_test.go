package backport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/proposal"
)

// bundle is a workspace where PR 12 added b.txt, PR 15 changed build.yaml and PR 18 added c.txt,
// all to main, in that order. release/v0 (v2.14) takes all three cleanly; release/v1 (v2.15)
// changed build.yaml itself, so PR 15 conflicts there.
func bundle(t *testing.T) (*backWS, *proposal.Executor) {
	t.Helper()
	ws, ex := newBackWS(t)
	ws.mergedPR(15, ws.newImage, "Update image")
	ws.mergedPR(18, ws.addC, "Add c")
	return ws, ex
}

func TestBundleMakesOnePRForTheTargetWithBothPRsLinkedByTitle(t *testing.T) {
	ws, ex := bundle(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.NoError(t, err)
	require.Len(t, plan.PRs, 2)
	assert.Equal(t, 12, plan.PRs[0].Number)
	assert.Equal(t, []string{ws.addB}, plan.PRs[0].Picks)
	assert.Equal(t, 15, plan.PRs[1].Number)
	assert.Equal(t, []string{ws.newImage}, plan.PRs[1].Picks)
	require.Len(t, plan.Targets, 1, "one PR for the target, not one for each source PR")
	tg := plan.Targets[0]
	assert.Equal(t, []int{12, 15}, tg.PRs)
	assert.Empty(t, tg.Skipped)
	assert.Equal(t, []string{ws.addB, ws.newImage}, tg.Picks)
	require.NotNil(t, tg.Preview)
	assert.Equal(t, "[Backport v2.14] Backport of: #12, #15", tg.Preview.Title)
	assert.Contains(t, tg.Preview.Body, "[Add b](https://github.com/acme/cons/pull/12)")
	assert.Contains(t, tg.Preview.Body, "[Update image](https://github.com/acme/cons/pull/15)")
	assert.True(t, strings.HasPrefix(tg.Preview.Branch, "bot/backport-12+15-"), tg.Preview.Branch)
	m, ok := proposal.ParseMarker(tg.Preview.Body)
	require.True(t, ok)
	assert.Equal(t, "backport-12+15", m.Source)
	require.NotNil(t, tg.Result)
	assert.Equal(t, proposal.ResultCommitted, tg.Result.Status)
	assert.Len(t, tg.Result.Commits, 2)
	assert.Equal(t, "Add b", strings.TrimSpace(ws.onBranch(tg.Preview.Branch, "b.txt")))
	assert.Equal(t, "image: new", strings.TrimSpace(ws.onBranch(tg.Preview.Branch, "build.yaml")))
}

func TestBundleIsPickedInTheOrderOfTheBaseWhateverTheOrderTyped(t *testing.T) {
	ws, ex := bundle(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{18, 12, 15}, NoPush: true}, ex)

	require.NoError(t, err)
	tg := plan.Targets[0]
	assert.Equal(t, []string{ws.addB, ws.newImage, ws.addC}, tg.Picks, "the order they were applied on main")
	assert.Equal(t, []int{12, 15, 18}, tg.PRs)
	assert.Equal(t, []int{12, 15, 18}, []int{plan.PRs[0].Number, plan.PRs[1].Number, plan.PRs[2].Number})
	m, _ := proposal.ParseMarker(tg.Preview.Body)
	assert.Equal(t, "backport-12+15+18", m.Source, "the numbers of the identity are sorted")
}

func TestBundleOfTheSameSetTypedAnotherWayHasTheSameBranch(t *testing.T) {
	ws, ex := bundle(t)
	first, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true, DryRun: true}, ex)
	require.NoError(t, err)

	second, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{15, 12}, NoPush: true, DryRun: true}, ex)

	require.NoError(t, err)
	assert.Equal(t, first.Targets[0].Preview.Branch, second.Targets[0].Preview.Branch)
}

func TestBundleCountsAPRNamedTwiceOnce(t *testing.T) {
	ws, ex := bundle(t)

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 12}, NoPush: true}, ex)

	require.NoError(t, err)
	assert.Equal(t, []int{12}, plan.Targets[0].PRs)
	assert.Equal(t, "[Backport v2.14] Add b", plan.Targets[0].Preview.Title, "one PR is not a bundle")
}

func TestBundleRefusesPRsMergedToDifferentBases(t *testing.T) {
	ws, ex := bundle(t)
	d := ws.host.details[15]
	d.Base = "release/v1"
	ws.host.details[15] = d

	_, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.ErrorContains(t, err, "#15")
	require.ErrorContains(t, err, "release/v1")
	require.ErrorContains(t, err, "one base")
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"))
}

func TestBundleRefusesAPRThatIsNotMergedAndNamesIt(t *testing.T) {
	ws, ex := bundle(t)
	d := ws.host.details[15]
	d.Merged = false
	ws.host.details[15] = d

	_, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.ErrorContains(t, err, "acme/cons#15 is not merged")
}

func TestBundleRefusesMoreThanTheMostItTakes(t *testing.T) {
	ws, ex := bundle(t)
	var many []int
	for n := 1; n <= MaxPRs+1; n++ {
		many = append(many, n)
	}

	_, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: many, NoPush: true}, ex)

	require.ErrorContains(t, err, "10")
}

func TestBundleLeavesOutAPRTheTargetHasByItsOriginLine(t *testing.T) {
	ws, ex := bundle(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v0")
	runGit(t, ws.up, "cherry-pick", "-x", ws.addB)
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.NoError(t, err)
	tg := plan.Targets[0]
	assert.Equal(t, []int{15}, tg.PRs)
	assert.Equal(t, []int{12}, tg.Skipped)
	assert.Equal(t, []string{ws.newImage}, tg.Picks)
	assert.Equal(t, "[Backport v2.14] Update image", tg.Preview.Title, "one PR left is the regular form")
	assert.Contains(t, tg.Preview.Body, "Already on this branch")
	assert.Contains(t, tg.Preview.Body, "#12")
	m, _ := proposal.ParseMarker(tg.Preview.Body)
	assert.Equal(t, "backport-15", m.Source)
	require.NotNil(t, tg.Result)
	assert.Equal(t, proposal.ResultCommitted, tg.Result.Status)
}

func TestBundleLeavesOutAPRTheTargetHasByItsPatchWhoeverMadeIt(t *testing.T) {
	ws, ex := bundle(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v0")
	runGit(t, ws.up, "cherry-pick", ws.addB) // by hand, with no origin line
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.NoError(t, err)
	tg := plan.Targets[0]
	assert.Equal(t, []int{15}, tg.PRs)
	assert.Equal(t, []int{12}, tg.Skipped)
}

func TestBundleOfPRsTheTargetAllHasIsAlreadyBackportedAndMakesNoPR(t *testing.T) {
	ws, ex := bundle(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v0")
	runGit(t, ws.up, "cherry-pick", "-x", ws.addB, ws.newImage)
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.14"}, PRs: []int{12, 15}, NoPush: true}, ex)

	require.NoError(t, err)
	tg := plan.Targets[0]
	assert.Equal(t, StatusAlreadyBackported, tg.Status)
	assert.Equal(t, []int{12, 15}, tg.Skipped)
	assert.Empty(t, tg.PRs)
	assert.Nil(t, tg.Result)
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"))
}

func TestBundleDecidesWhatIsLeftOutForEachTargetOnItsOwn(t *testing.T) {
	ws, ex := bundle(t)
	runGit(t, ws.up, "checkout", "--quiet", "release/v1")
	runGit(t, ws.up, "cherry-pick", "-x", ws.addB)
	runGit(t, ws.up, "checkout", "--quiet", "main")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12, 18}, NoPush: true}, ex)

	require.NoError(t, err)
	one, both := target(t, plan, "release/v1"), target(t, plan, "release/v0")
	assert.Equal(t, []int{18}, one.PRs)
	assert.Equal(t, []int{12}, one.Skipped)
	assert.Equal(t, "[Backport v2.15] Add c", one.Preview.Title)
	assert.Equal(t, []int{12, 18}, both.PRs)
	assert.Equal(t, "[Backport v2.14] Backport of: #12, #18", both.Preview.Title)
}

func TestBundleConflictNamesThePRWhosePickStoppedAndTheBodyNamesItsFile(t *testing.T) {
	ws, ex := bundle(t)
	resolver := resolving("image: merged\n")

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15"}, PRs: []int{12, 15}, NoPush: true, Resolver: resolver}, ex)

	require.NoError(t, err)
	require.Len(t, resolver.seen, 1)
	assert.Equal(t, 15, resolver.seen[0].PR, "PR 12 picked cleanly; it is PR 15 that stopped")
	assert.Equal(t, ws.newImage, resolver.seen[0].Commit)
	tg := target(t, plan, "release/v1")
	require.NotNil(t, tg.Result)
	assert.Len(t, tg.Result.Commits, 2)
	assert.Contains(t, tg.Preview.Body, "`build.yaml` (#15)")
	assert.Equal(t, "main", current(t, ws.clone))
}

func TestBundleAbandonedAtAConflictLeavesNoneOfItsPicks(t *testing.T) {
	ws, ex := bundle(t)
	resolver := &scripted{do: func(Conflict) (Choice, error) { return Abort, nil }}

	plan, err := Run(bg, ws, Input{Lines: []string{"v2.15", "v2.14"}, PRs: []int{12, 15}, NoPush: true, Resolver: resolver}, ex)

	require.ErrorIs(t, err, ErrAborted)
	assert.Equal(t, StatusAborted, target(t, plan, "release/v1").Status)
	assert.Equal(t, StatusNotAttempted, target(t, plan, "release/v0").Status)
	assert.Empty(t, runGit(t, ws.clone, "branch", "--list", "bot/*"), "PR 12's clean pick went with the abandoned target")
	assert.Empty(t, runGit(t, ws.clone, "status", "--porcelain"))
}
