package backport

import (
	"testing"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func plan() domain.Plan {
	return domain.Plan{
		Repo: "rancher/backup-restore-operator", Base: "main",
		PRs: []domain.Source{
			{Number: 42, Title: "Fix the thing", URL: "https://github.com/rancher/backup-restore-operator/pull/42", Shape: domain.ShapeRebase, Picks: []string{"1111111aaaa", "2222222bbbb"}},
			{Number: 43, Title: "Follow-up", URL: "https://github.com/rancher/backup-restore-operator/pull/43", Shape: domain.ShapeSquash, Picks: []string{"4444444dddd"}},
		},
		Targets: []domain.Target{
			{Branch: "release/v7.x", Line: "v2.15", Status: domain.StatusReady, Files: []string{"build.yaml"}, PRs: []int{42, 43},
				Preview: &proposal.Preview{Branch: "bot/backport-42-ab12cd34-v7.x", Title: "[Backport v2.15] Fix the thing", Body: "## Summary\nthe body"},
				Result: &proposal.TargetResult{Status: proposal.ResultOpened, Branch: "bot/backport-42-ab12cd34-v7.x",
					PR:      &proposal.PRRef{Number: 77, URL: "https://github.com/rancher/backup-restore-operator/pull/77"},
					Commits: []proposal.CommitInfo{{Hash: "3333333cccc", Subject: "Fix part one"}}}},
			{Branch: "release/v6.x", Line: "v2.14", Status: domain.StatusAlreadyBackported},
		},
	}
}

func TestRenderTextSaysWhatThePRIsHowItLandedAndWhatEachTargetGot(t *testing.T) {
	out := renderText(plan(), false, style.Plain())

	for _, want := range []string{
		"Backport: rancher/backup-restore-operator", "#42 Fix the thing", "merged to main as a rebase: picks 1111111, 2222222",
		"#43 Follow-up", "as a squash: picks 4444444",
		"v2.15", "release/v7.x", "carries #42, #43", "branch  bot/backport-42-ab12cd34-v7.x", "title   [Backport v2.15] Fix the thing",
		"opened #77 https://github.com/rancher/backup-restore-operator/pull/77", "commit 3333333 Fix part one",
		"resolved by hand: build.yaml",
		"v2.14", "already backported",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "## Summary", "the body is for --preview")
}

func TestRenderTextOfASinglePRSaysNothingOfABundle(t *testing.T) {
	p := plan()
	p.PRs = p.PRs[:1]
	p.Targets[0].PRs = []int{42}

	out := renderText(p, false, style.Plain())

	assert.NotContains(t, out, "carries")
}

func TestRenderTextNamesThePRsATargetLeftOutBecauseItHadThem(t *testing.T) {
	p := plan()
	p.Targets[0].PRs, p.Targets[0].Skipped = []int{43}, []int{42}

	out := renderText(p, false, style.Plain())

	assert.Contains(t, out, "already on the branch: #42")
}

func TestRenderTextWithPreviewHasTheBody(t *testing.T) {
	assert.Contains(t, renderText(plan(), true, style.Plain()), "## Summary")
}

func TestRenderTextSaysWhichTargetsWereAbandonedAndWhichWereNotTried(t *testing.T) {
	p := plan()
	p.Targets = []domain.Target{
		{Branch: "release/v7.x", Line: "v2.15", Status: domain.StatusAborted, Result: &proposal.TargetResult{Status: proposal.ResultFailed, Error: "the cherry-pick was abandoned: 1111111 conflicts in build.yaml"}},
		{Branch: "release/v6.x", Line: "v2.14", Status: domain.StatusNotAttempted},
	}

	out := renderText(p, false, style.Plain())

	assert.Contains(t, out, "aborted")
	assert.Contains(t, out, "conflicts in build.yaml")
	assert.Contains(t, out, "not attempted")
}

func TestFailedIsAnErrorWhenATargetDidNotGetItsBackport(t *testing.T) {
	assert.NoError(t, failed(plan()), "opened and already backported are both done")
	for name, status := range map[string]domain.Status{"aborted": domain.StatusAborted, "not attempted": domain.StatusNotAttempted} {
		p := plan()
		p.Targets[1].Status = status
		require.ErrorIs(t, failed(p), ErrFailed, name)
	}
	p := plan()
	p.Targets[0].Result.Status = proposal.ResultFailed
	require.ErrorIs(t, failed(p), ErrFailed, "a failed result")
	p = plan()
	p.Targets[0].Result.Status = proposal.ResultBlocked
	require.ErrorIs(t, failed(p), ErrFailed, "blocked by a newer PR")
}
