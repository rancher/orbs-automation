package reposync

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/proposal"
	domain "github.com/rancher/orbs-automation/internal/reposync"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func syncPlan() domain.Plan {
	row := func(l, branch, holding, current, want string, st domain.Status) domain.Row {
		return domain.Row{
			SeriesLine: domain.SeriesLine{Line: l, Branch: branch, Current: current, Want: want, UpToDate: current == want},
			Holding:    holding, Status: st,
		}
	}
	blocked := row("v2.14", "release/v6.x", "holds-kuberlr", "v6", "v7", domain.StatusBlocked)
	blocked.Reason = "no Edit follows the holding: Holding is only read until one does"
	return domain.Plan{Holder: "rancher/prometheus-federator", Branches: []domain.Branch{
		{Branch: "main", Status: domain.StatusUpToDate, Rows: []domain.Row{row("v2.16", "main", "holds-kuberlr", "v9", "v9", domain.StatusUpToDate)}},
		{Branch: "release/v7.x", Status: domain.StatusReady,
			Rows: []domain.Row{
				row("v2.15", "release/v7.x", "holds-kuberlr", "v9", "v8", domain.StatusReady),
				row("v2.15", "release/v7.x", "holds-other", "r4", "r3", domain.StatusReady),
			},
			Preview: &proposal.Preview{Branch: "bot/sync-1a2b3c4d-v7.x", Title: "[v7.x] Sync holds-kuberlr, holds-other on v2.15", CommitMessage: "Sync holds-kuberlr on v2.15\n\nAutomation: gero ship", Body: "## Summary\nthe body"},
			Result:  &proposal.TargetResult{Status: proposal.ResultOpened, Branch: "bot/sync-1a2b3c4d-v7.x", PR: &proposal.PRRef{Number: 12, URL: "https://github.com/rancher/prometheus-federator/pull/12"}}},
		{Branch: "release/v6.x", Status: domain.StatusBlocked, Rows: []domain.Row{blocked}},
	}}
}

func TestRenderSyncTextHasARowForEachHoldingAndTheWorkDoneForEachBranch(t *testing.T) {
	out := renderSyncText(syncPlan(), false, style.Plain())

	for _, want := range []string{
		"Sync: rancher/prometheus-federator", "LINE", "HOLDS", "WANTS", "holds-other",
		"current", "differs, ready", "differs, left alone: no Edit follows the holding",
		"release/v7.x\n", "branch  bot/sync-1a2b3c4d-v7.x", "title   [v7.x] Sync holds-kuberlr, holds-other on v2.15",
		"opened #12 https://github.com/rancher/prometheus-federator/pull/12",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "## Summary", "the body is for --preview")
}

func TestRenderSyncTextWithPreviewHasTheCommitMessageAndTheBody(t *testing.T) {
	out := renderSyncText(syncPlan(), true, style.Plain())

	assert.Contains(t, out, "Automation: gero ship")
	assert.Contains(t, out, "## Summary")
}

func TestRenderSyncTextOfAProjectWithNothingToSyncSaysSo(t *testing.T) {
	out := renderSyncText(domain.Plan{Holder: "rancher/rancher"}, false, style.Plain())

	assert.Contains(t, out, "nothing to sync")
}

func TestASyncFailsOnlyWhenABranchOrARowDid(t *testing.T) {
	require.NoError(t, syncFailed(syncPlan()), "a row left alone for want of an Edit is reported, not failed")

	p := syncPlan()
	p.Branches[1].Result = &proposal.TargetResult{Status: proposal.ResultFailed}
	require.ErrorIs(t, syncFailed(p), ErrFailed)

	p = syncPlan()
	p.Branches[0].Rows[0].Status = domain.StatusFailed
	require.ErrorIs(t, syncFailed(p), ErrFailed, "a branch that could not be read")
}
