package ship

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logged plays events through a log sink with a JSON handler and returns the
// records.
func logged(t *testing.T, level slog.Level, events ...domain.Event) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	sink := logSink{slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))}
	for _, e := range events {
		sink.Event(e)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &rec), line)
		out = append(out, rec)
	}
	return out
}

func TestLogSinkRecordsEachEventWithItsAttributes(t *testing.T) {
	p := previewPlan()
	header := p
	header.Targets = nil
	target := p.Targets[0]

	recs := logged(t, slog.LevelInfo,
		domain.Event{Kind: domain.EventResolved, Plan: &header},
		domain.Event{Kind: domain.EventTargetChecked, Branch: "release/v2.15", Target: &target},
		domain.Event{Kind: domain.EventImagesChecked, Images: &registry.Report{Rows: []registry.Row{{Status: "found"}}}},
		domain.Event{Kind: domain.EventTargetStarted, Branch: "release/v2.15", Place: domain.Worktree, Reason: "a dry run never moves your checkout"},
		domain.Event{Kind: domain.EventCommit, Branch: "release/v2.15", Commit: &domain.CommitInfo{Hash: "abcdef0123", Subject: "Update"}},
		domain.Event{Kind: domain.EventTargetDone, Branch: "release/v2.15", Result: &domain.TargetResult{Status: domain.ResultCommitted, Branch: "bot/x"}},
		domain.Event{Kind: domain.EventDone, Plan: &p},
	)

	require.Len(t, recs, 7)
	assert.Equal(t, "ship resolved", recs[0]["msg"])
	assert.Equal(t, "rancher/scc-operator", recs[0]["producer"])
	assert.Equal(t, "v0.5.3", recs[0]["release"])
	assert.Equal(t, "rancher/rancher", recs[0]["consumer"])
	assert.Equal(t, []any{"rancher-holds-scc"}, recs[0]["holdings"])
	assert.Equal(t, "target checked", recs[1]["msg"])
	assert.Equal(t, "release/v2.15", recs[1]["branch"])
	assert.Equal(t, "ready", recs[1]["status"])
	assert.Equal(t, "a", recs[1]["current"])
	assert.Equal(t, "images checked", recs[2]["msg"])
	assert.EqualValues(t, 1, recs[2]["rows"])
	assert.Equal(t, false, recs[2]["failed"])
	assert.Equal(t, "target started", recs[3]["msg"])
	assert.Equal(t, "worktree", recs[3]["place"])
	assert.Equal(t, "a dry run never moves your checkout", recs[3]["reason"])
	assert.Equal(t, "commit", recs[4]["msg"])
	assert.Equal(t, "abcdef0123", recs[4]["commit"])
	assert.Equal(t, "Update", recs[4]["subject"])
	assert.Equal(t, "target done", recs[5]["msg"])
	assert.Equal(t, "committed", recs[5]["status"])
	assert.Equal(t, "bot/x", recs[5]["new_branch"])
	assert.Equal(t, "ship done", recs[6]["msg"])
	assert.Equal(t, "ready", recs[6]["status"])
	for _, rec := range recs {
		assert.Equal(t, "INFO", rec["level"], "progress is info: the default level hides it, and -v shows it")
	}
}

func TestLogSinkAFailedTargetIsAWarningWithTheError(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{
		Kind: domain.EventTargetDone, Branch: "release/v2.15",
		Result: &domain.TargetResult{Status: domain.ResultFailed, Error: "method postEdit[0] ./gen: exit status 1"},
	})

	require.Len(t, recs, 1)
	assert.Equal(t, "WARN", recs[0]["level"])
	assert.Equal(t, "method postEdit[0] ./gen: exit status 1", recs[0]["error"])
}

func TestLogSinkAFailedImageCheckIsAWarning(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{
		Kind: domain.EventImagesChecked, Images: &registry.Report{Rows: []registry.Row{{Status: "not found"}}},
	})

	require.Len(t, recs, 1)
	assert.Equal(t, "WARN", recs[0]["level"])
	assert.Equal(t, true, recs[0]["failed"])
}

func TestLogSinkHasNoRecordForTheEventThatOnlyEndsATable(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventTargetsChecked})

	assert.Empty(t, recs)
}

func TestLogSinkRespectsTheLevelOfTheHandler(t *testing.T) {
	p := previewPlan()
	recs := logged(t, slog.LevelWarn, domain.Event{Kind: domain.EventDone, Plan: &p})

	assert.Empty(t, recs, "at the default level the progress is not shown")
}

type countingSink struct{ n int }

func (c *countingSink) Event(domain.Event) { c.n++ }

func TestSinksSendEachEventToEverySinkInOrderAndSkipNil(t *testing.T) {
	a, b := &countingSink{}, &countingSink{}

	sinks{a, nil, b}.Event(domain.Event{Kind: domain.EventDone})
	sinks{a}.Event(domain.Event{Kind: domain.EventDone})
	sinks(nil).Event(domain.Event{Kind: domain.EventDone})

	assert.Equal(t, 2, a.n)
	assert.Equal(t, 1, b.n)
}

func TestLogSinkRecordsTheDecisionAndEachStepOfPublishing(t *testing.T) {
	recs := logged(t, slog.LevelInfo,
		domain.Event{Kind: domain.EventDecided, Branch: "release/v2.15", Decision: &domain.Publish{
			Action: domain.PublishCreate, Supersede: []pulls.PR{{Number: 101}, {Number: 90}},
		}},
		domain.Event{Kind: domain.EventPushed, Branch: "release/v2.15", Remote: "origin"},
		domain.Event{Kind: domain.EventPROpened, Branch: "release/v2.15", PR: &domain.PRRef{Number: 104, URL: "https://github.com/rancher/rancher/pull/104"}},
		domain.Event{Kind: domain.EventSuperseded, Branch: "release/v2.15", PR: &domain.PRRef{Number: 101}},
	)

	require.Len(t, recs, 4)
	assert.Equal(t, "decided", recs[0]["msg"])
	assert.Equal(t, "create", recs[0]["action"])
	assert.EqualValues(t, 2, recs[0]["supersede"])
	assert.Equal(t, "pushed", recs[1]["msg"])
	assert.Equal(t, "origin", recs[1]["remote"])
	assert.Equal(t, "pr opened", recs[2]["msg"])
	assert.EqualValues(t, 104, recs[2]["pr"])
	assert.Equal(t, "https://github.com/rancher/rancher/pull/104", recs[2]["url"])
	assert.Equal(t, "superseded", recs[3]["msg"])
	assert.EqualValues(t, 101, recs[3]["pr"])
	for _, rec := range recs {
		assert.Equal(t, "INFO", rec["level"])
		assert.Equal(t, "release/v2.15", rec["branch"])
	}
}

func TestLogSinkABlockedDecisionIsAWarning(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventDecided, Branch: "release/v2.15", Decision: &domain.Publish{
		Action: domain.PublishBlocked, Newer: &pulls.PR{Number: 110},
	}})

	require.Len(t, recs, 1)
	assert.Equal(t, "WARN", recs[0]["level"])
	assert.Equal(t, "blocked", recs[0]["action"])
	assert.EqualValues(t, 110, recs[0]["newer"])
}

func TestLogSinkATargetDoneRecordsThePRAndTheWarnings(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventTargetDone, Branch: "release/v2.15", Result: &domain.TargetResult{
		Status: domain.ResultOpened, Branch: "bot/x", PR: &domain.PRRef{Number: 104}, Warnings: []string{"a", "b"},
	}})

	require.Len(t, recs, 1)
	assert.EqualValues(t, 104, recs[0]["pr"])
	assert.EqualValues(t, 2, recs[0]["warnings"])
}

func TestLogSinkRecordsARefresh(t *testing.T) {
	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventRefreshed, Branch: "release/v2.15", PR: &domain.PRRef{Number: 104, URL: "u"}})

	require.Len(t, recs, 1)
	assert.Equal(t, "refreshed", recs[0]["msg"])
	assert.EqualValues(t, 104, recs[0]["pr"])
	assert.Equal(t, "release/v2.15", recs[0]["branch"])
}

func TestLogSinkRecordsWhoTheRunActsAs(t *testing.T) {
	a := &domain.Actor{Identity: domain.Identity{Name: "Dan Pock", Email: "dan@example.com"}, Login: "danpock", Signing: true}

	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventActor, Actor: a})

	require.Len(t, recs, 1)
	assert.Equal(t, "acting as", recs[0]["msg"])
	assert.Equal(t, "dan@example.com", recs[0]["email"])
	assert.Equal(t, "danpock", recs[0]["login"])
	assert.Equal(t, true, recs[0]["signing"])
}

func TestLogSinkRecordsHowManyFilesACommitChanged(t *testing.T) {
	c := &domain.CommitInfo{Hash: "abcdef0123", Subject: "Update", Files: []string{"build.yaml", "go.mod"}}

	recs := logged(t, slog.LevelInfo, domain.Event{Kind: domain.EventCommit, Branch: "release/v2.15", Commit: c})

	require.Len(t, recs, 1)
	assert.EqualValues(t, 2, recs[0]["files"])
}
