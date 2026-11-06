package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kinds(events []domain.Event) []domain.EventKind {
	out := make([]domain.EventKind, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func TestReplayOfACheckOnlyPlan(t *testing.T) {
	p := previewPlan()
	p.Images = &registry.Report{}

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked, domain.EventImagesChecked, domain.EventDone,
	}, kinds(events))
	require.Len(t, events, 5)
	require.NotNil(t, events[0].Plan)
	assert.Equal(t, "rancher/scc-operator", events[0].Plan.Producer)
	assert.Equal(t, "v0.5.3", events[0].Plan.Release.Tag)
	assert.Empty(t, events[0].Plan.Targets, "the header only, as a run emits it")
	require.NotNil(t, events[1].Target)
	assert.Equal(t, "release/v2.15", events[1].Target.Branch)
	assert.Equal(t, "release/v2.15", events[1].Branch)
	require.NotNil(t, events[4].Plan)
	assert.Len(t, events[4].Plan.Targets, 1, "the whole plan at the end")
}

func TestReplayHasNoImagesEventWhenTheyWereNotChecked(t *testing.T) {
	p := previewPlan()

	assert.NotContains(t, kinds(replay(p)), domain.EventImagesChecked)
}

func TestReplayOfARunThatCommitted(t *testing.T) {
	p := previewPlan()
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")
	r.Commits = []domain.CommitInfo{{Hash: "abcdef0", Subject: "one"}, {Hash: "1234567", Subject: "two"}}
	p = withResults(p, r)

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked,
		domain.EventTargetStarted, domain.EventCommit, domain.EventCommit, domain.EventTargetDone, domain.EventDone,
	}, kinds(events))
	require.Len(t, events, 8)
	assert.Equal(t, domain.InPlace, events[3].Place)
	assert.Equal(t, "release/v2.15", events[3].Branch)
	require.NotNil(t, events[4].Commit)
	assert.Equal(t, "one", events[4].Commit.Subject)
	assert.Equal(t, "two", events[5].Commit.Subject)
	require.NotNil(t, events[6].Result)
	assert.Equal(t, domain.ResultCommitted, events[6].Result.Status)
}

func TestReplayOfATargetThatFailedBeforeItStartedHasNoStartedEvent(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultFailed, Branch: "bot/x", Error: "moved"})

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked, domain.EventTargetDone, domain.EventDone,
	}, kinds(events), "no place was chosen, so the work never started")
}

func TestReplayOfAPublishedTargetHasTheStepsOfPublishing(t *testing.T) {
	p := previewPlan()
	r := committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")
	r.Status = domain.ResultOpened
	r.Pushed = "origin"
	r.PR = &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"}
	r.Superseded = []domain.PRRef{{Number: 101}, {Number: 90}}
	p = withResults(p, r)

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked,
		domain.EventTargetStarted, domain.EventCommit, domain.EventPushed, domain.EventPROpened,
		domain.EventSuperseded, domain.EventSuperseded, domain.EventTargetDone, domain.EventDone,
	}, kinds(events))
	require.Len(t, events, 11)
	assert.Equal(t, "origin", events[5].Remote)
	require.NotNil(t, events[6].PR)
	assert.Equal(t, 104, events[6].PR.Number)
	assert.Equal(t, 101, events[7].PR.Number)
	assert.Equal(t, 90, events[8].PR.Number)
}

func TestReplayOfAnAlreadyOpenTargetHasNoStartedAndNoCommits(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultAlreadyOpen, Branch: "bot/x", PR: &domain.PRRef{Number: 104}, Superseded: []domain.PRRef{{Number: 101}}})

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked,
		domain.EventSuperseded, domain.EventTargetDone, domain.EventDone,
	}, kinds(events), "it was already open: the older one was closed and nothing else was made")
}
