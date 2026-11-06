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
		domain.EventResolved, domain.EventTargetChecked, domain.EventImagesChecked, domain.EventDone,
	}, kinds(events))
	require.Len(t, events, 4)
	require.NotNil(t, events[0].Plan)
	assert.Equal(t, "rancher/scc-operator", events[0].Plan.Producer)
	assert.Equal(t, "v0.5.3", events[0].Plan.Release.Tag)
	assert.Empty(t, events[0].Plan.Targets, "the header only, as a run emits it")
	require.NotNil(t, events[1].Target)
	assert.Equal(t, "release/v2.15", events[1].Target.Branch)
	assert.Equal(t, "release/v2.15", events[1].Branch)
	require.NotNil(t, events[3].Plan)
	assert.Len(t, events[3].Plan.Targets, 1, "the whole plan at the end")
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
		domain.EventResolved, domain.EventTargetChecked,
		domain.EventTargetStarted, domain.EventCommit, domain.EventCommit, domain.EventTargetDone, domain.EventDone,
	}, kinds(events))
	require.Len(t, events, 7)
	assert.Equal(t, domain.InPlace, events[2].Place)
	assert.Equal(t, "release/v2.15", events[2].Branch)
	require.NotNil(t, events[3].Commit)
	assert.Equal(t, "one", events[3].Commit.Subject)
	assert.Equal(t, "two", events[4].Commit.Subject)
	require.NotNil(t, events[5].Result)
	assert.Equal(t, domain.ResultCommitted, events[5].Result.Status)
}

func TestReplayOfATargetThatFailedBeforeItStartedHasNoStartedEvent(t *testing.T) {
	p := previewPlan()
	p = withResults(p, &domain.TargetResult{Status: domain.ResultFailed, Branch: "bot/x", Error: "moved"})

	events := replay(p)

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetDone, domain.EventDone,
	}, kinds(events), "no place was chosen, so the work never started")
}
