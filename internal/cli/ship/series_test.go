package ship

import (
	"testing"

	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seriesPlan() domain.SeriesPlan {
	line := func(l, branch, current, want string) domain.SeriesLine {
		return domain.SeriesLine{Line: l, Branch: branch, Current: current, Want: want, UpToDate: current == want}
	}
	return domain.SeriesPlan{Holder: "rancher/prometheus-federator", Targets: []domain.SeriesTarget{
		{SeriesLine: line("v2.16", "main", "v9", "v9"), Holding: "holds-kuberlr", Status: domain.StatusUpToDate},
		{SeriesLine: line("v2.15", "release/v7.x", "v9", "v8"), Holding: "holds-kuberlr", Status: domain.StatusReady,
			Preview: &domain.Preview{Branch: "bot/holds-kuberlr-v8-v7.x", Title: "[v7.x] Track kuberlr 8.x on v2.15", CommitMessage: "Track kuberlr 8.x on v2.15\n\nAutomation: gero ship", Body: "## Summary\nthe body"},
			Result:  &domain.TargetResult{Status: domain.ResultOpened, Branch: "bot/holds-kuberlr-v8-v7.x", PR: &domain.PRRef{Number: 12, URL: "https://github.com/rancher/prometheus-federator/pull/12"}}},
		{SeriesLine: line("v2.14", "release/v6.x", "v6", "v7"), Holding: "holds-kuberlr", Status: domain.StatusBlocked, Reason: "no Edit follows the holding: Holding is only read until one does"},
	}}
}

func TestRenderSeriesTextHasARowForEachLineAndTheWorkDone(t *testing.T) {
	out := renderSeriesText(seriesPlan(), false, style.Plain())

	for _, want := range []string{
		"Series: rancher/prometheus-federator", "LINE", "HOLDS", "WANTS",
		"current", "differs, ready", "differs, left alone: no Edit follows the holding",
		"v2.15 on release/v7.x", "branch  bot/holds-kuberlr-v8-v7.x", "title   [v7.x] Track kuberlr 8.x on v2.15",
		"opened #12 https://github.com/rancher/prometheus-federator/pull/12",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "## Summary", "the body is for --preview")
}

func TestRenderSeriesTextWithPreviewHasTheCommitMessageAndTheBody(t *testing.T) {
	out := renderSeriesText(seriesPlan(), true, style.Plain())

	assert.Contains(t, out, "Automation: gero ship")
	assert.Contains(t, out, "## Summary")
}

func TestRenderSeriesTextOfAProjectWithNoSeriesHoldingSaysSo(t *testing.T) {
	out := renderSeriesText(domain.SeriesPlan{Holder: "rancher/rancher"}, false, style.Plain())

	assert.Contains(t, out, "no series holding")
}

func TestASeriesRunFailsOnlyWhenATargetDid(t *testing.T) {
	require.NoError(t, seriesFailed(seriesPlan()), "a line left alone for want of an Edit is reported, not failed")

	p := seriesPlan()
	p.Targets[1].Result = &domain.TargetResult{Status: domain.ResultFailed}
	require.ErrorIs(t, seriesFailed(p), ErrFailed)

	p = seriesPlan()
	p.Targets[0].Status = domain.StatusFailed
	require.ErrorIs(t, seriesFailed(p), ErrFailed, "a branch that could not be read")
}
