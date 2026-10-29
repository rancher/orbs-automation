package doctor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCheck is a Check driven by its fields. It records into ran when it runs.
type fakeCheck struct {
	name, category string
	online         bool
	needs          []string
	run            func(ctx context.Context) Result
	ran            *[]string
}

func (f fakeCheck) Name() string        { return f.name }
func (f fakeCheck) Category() string    { return f.category }
func (f fakeCheck) Description() string { return "checks " + f.name }
func (f fakeCheck) Online() bool        { return f.online }
func (f fakeCheck) Needs() []string     { return f.needs }
func (f fakeCheck) Run(ctx context.Context) Result {
	if f.ran != nil {
		*f.ran = append(*f.ran, f.name)
	}
	if f.run != nil {
		return f.run(ctx)
	}
	return Result{Status: StatusOK, Message: f.name + " is fine"}
}

func result(s Status) func(context.Context) Result {
	return func(context.Context) Result { return Result{Status: s, Message: "msg", Remediation: "fix it"} }
}

func names(os []Outcome) []string {
	out := make([]string, len(os))
	for i, o := range os {
		out[i] = o.Name
	}
	return out
}

func statuses(os []Outcome) []Status {
	out := make([]Status, len(os))
	for i, o := range os {
		out[i] = o.Status
	}
	return out
}

func TestStatusString(t *testing.T) {
	assert.Equal(t, "ok", StatusOK.String())
	assert.Equal(t, "warn", StatusWarn.String())
	assert.Equal(t, "fail", StatusFail.String())
	assert.Equal(t, "skip", StatusSkip.String())
}

func TestRunKeepsRegistrationOrderAndCopiesTheResult(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "b", category: "dependencies", ran: &ran, run: result(StatusWarn)},
		fakeCheck{name: "a", category: "configuration", ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{})

	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, ran)
	require.Len(t, got, 2)
	assert.Equal(t, Outcome{Name: "b", Category: "dependencies", Status: StatusWarn, Message: "msg", Remediation: "fix it"}, got[0])
	assert.Equal(t, Outcome{Name: "a", Category: "configuration", Status: StatusOK, Message: "a is fine"}, got[1])
}

func TestRunSkipsOnlineChecksUnlessAsked(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "local", ran: &ran},
		fakeCheck{name: "remote", online: true, ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{})

	require.NoError(t, err)
	assert.Equal(t, []string{"local"}, ran, "an online check must not run offline")
	require.Len(t, got, 2)
	assert.Equal(t, Outcome{Name: "remote", Online: true, Status: StatusSkip, SkipReason: "run with --online"}, got[1])

	ran = nil
	got, err = Run(context.Background(), checks, Options{Online: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"local", "remote"}, ran)
	assert.Equal(t, []Status{StatusOK, StatusOK}, statuses(got))
}

func TestRunSkipsWhatNeedsAFailedOrSkippedCheck(t *testing.T) {
	tests := []struct {
		name       string
		first      func(context.Context) Result
		firstOnl   bool
		wantStatus Status
		wantReason string
		wantRan    bool
	}{
		{name: "failed", first: result(StatusFail), wantStatus: StatusSkip, wantReason: "needs first"},
		{name: "skipped", first: result(StatusOK), firstOnl: true, wantStatus: StatusSkip, wantReason: "needs first"},
		{name: "warned", first: result(StatusWarn), wantStatus: StatusOK, wantRan: true},
		{name: "ok", first: result(StatusOK), wantStatus: StatusOK, wantRan: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ran []string
			checks := []Check{
				fakeCheck{name: "first", online: tt.firstOnl, run: tt.first, ran: &ran},
				fakeCheck{name: "second", needs: []string{"first"}, ran: &ran},
			}

			got, err := Run(context.Background(), checks, Options{})

			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, tt.wantStatus, got[1].Status)
			assert.Equal(t, tt.wantReason, got[1].SkipReason)
			assert.Equal(t, tt.wantRan, contains(ran, "second"))
		})
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestRunNamedChecksPullInWhatTheyNeed(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "a", ran: &ran},
		fakeCheck{name: "b", needs: []string{"a"}, ran: &ran},
		fakeCheck{name: "c", ran: &ran},
		fakeCheck{name: "d", needs: []string{"b"}, ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{Names: []string{"d"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "d"}, names(got), "the closure, in registration order")
	assert.Equal(t, []string{"a", "b", "d"}, ran)
}

func TestRunNamingAnOnlineCheckAllowsOnline(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "token", ran: &ran},
		fakeCheck{name: "api", online: true, needs: []string{"token"}, ran: &ran},
		fakeCheck{name: "limit", online: true, needs: []string{"api"}, ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{Names: []string{"limit"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"token", "api", "limit"}, ran, "no --online needed when an online check is named")
	assert.Equal(t, []Status{StatusOK, StatusOK, StatusOK}, statuses(got))
}

func TestRunNamingAnOfflineCheckDoesNotAllowOnline(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "api", online: true, ran: &ran},
		fakeCheck{name: "local", needs: []string{"api"}, ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{Names: []string{"local"}})

	require.NoError(t, err)
	assert.Empty(t, ran)
	require.Len(t, got, 2)
	assert.Equal(t, []Status{StatusSkip, StatusSkip}, statuses(got))
	assert.Equal(t, "run with --online", got[0].SkipReason)
	assert.Equal(t, "needs api", got[1].SkipReason)
}

func TestRunUnknownName(t *testing.T) {
	got, err := Run(context.Background(), []Check{fakeCheck{name: "a"}}, Options{Names: []string{"a", "nope"}})

	require.ErrorIs(t, err, ErrUnknownCheck)
	assert.ErrorContains(t, err, "nope")
	assert.Empty(t, got)
}

func TestRunRejectsAnInconsistentRegistry(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
	}{
		{"duplicate name", []Check{fakeCheck{name: "a"}, fakeCheck{name: "a"}}},
		{"unknown need", []Check{fakeCheck{name: "a", needs: []string{"ghost"}}}},
		{"need registered later", []Check{fakeCheck{name: "a", needs: []string{"b"}}, fakeCheck{name: "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Run(context.Background(), tt.checks, Options{})

			require.ErrorIs(t, err, ErrBadRegistry)
			assert.Empty(t, got)
		})
	}
}

func TestRunOnlineCheckGetsADeadline(t *testing.T) {
	var deadline time.Time
	var has bool
	check := fakeCheck{name: "api", online: true, run: func(ctx context.Context) Result {
		deadline, has = ctx.Deadline()
		return Result{Status: StatusOK}
	}}

	_, err := Run(context.Background(), []Check{check}, Options{Online: true})

	require.NoError(t, err)
	require.True(t, has, "an online check needs a deadline")
	assert.InDelta(t, DefaultTimeout.Seconds(), time.Until(deadline).Seconds(), 1)
}

func TestRunOfflineCheckGetsNoDeadline(t *testing.T) {
	var has, ran bool
	check := fakeCheck{name: "git", run: func(ctx context.Context) Result {
		ran = true
		_, has = ctx.Deadline()
		return Result{Status: StatusOK}
	}}

	_, err := Run(context.Background(), []Check{check}, Options{Online: true})

	require.NoError(t, err)
	require.True(t, ran)
	assert.False(t, has)
}

func TestRunTimeoutIsAWarning(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	var ran []string
	checks := []Check{
		// Ignores its context entirely, so the runner must not wait for it.
		fakeCheck{name: "slow", online: true, run: func(context.Context) Result { <-block; return Result{} }},
		fakeCheck{name: "after", ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{Online: true, Timeout: 20 * time.Millisecond})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, StatusWarn, got[0].Status)
	assert.Contains(t, got[0].Message, "timed out after 20ms")
	assert.NotEmpty(t, got[0].Remediation)
	assert.Equal(t, []string{"after"}, ran, "later checks still run")
}

func TestRunCheckThatHonorsTheDeadlineIsAWarningToo(t *testing.T) {
	check := fakeCheck{name: "api", online: true, run: func(ctx context.Context) Result {
		<-ctx.Done()
		return Result{Status: StatusFail, Message: ctx.Err().Error()}
	}}

	got, err := Run(context.Background(), []Check{check}, Options{Online: true, Timeout: 20 * time.Millisecond})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, StatusWarn, got[0].Status, "a flaky network is not a broken setup")
}

func TestRunPanicIsAFailureAndTheRunContinues(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "boom", run: func(context.Context) Result { panic("kaboom") }},
		fakeCheck{name: "after", ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, StatusFail, got[0].Status)
	assert.Contains(t, got[0].Message, "panicked")
	assert.Contains(t, got[0].Message, "kaboom")
	assert.Equal(t, []string{"after"}, ran)
}

func TestRunStopsWhenTheCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var ran []string
	checks := []Check{
		fakeCheck{name: "first", ran: &ran, run: func(context.Context) Result { cancel(); return Result{Status: StatusOK} }},
		fakeCheck{name: "second", ran: &ran},
	}

	_, err := Run(ctx, checks, Options{})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"first"}, ran)
}

func TestSummarize(t *testing.T) {
	os := []Outcome{
		{Status: StatusOK}, {Status: StatusOK}, {Status: StatusWarn},
		{Status: StatusFail}, {Status: StatusSkip}, {Status: StatusSkip},
	}

	assert.Equal(t, Counts{OK: 2, Warn: 1, Fail: 1, Skip: 2}, Summarize(os))
}

func TestNeedOnline(t *testing.T) {
	os := []Outcome{
		{Status: StatusSkip, SkipReason: "run with --online"},
		{Status: StatusSkip, SkipReason: "needs git"},
		{Status: StatusSkip, SkipReason: "run with --online"},
		{Status: StatusOK},
	}

	assert.Equal(t, 2, NeedOnline(os))
}

func TestFailed(t *testing.T) {
	ok := []Outcome{{Status: StatusOK}, {Status: StatusSkip}}
	warn := []Outcome{{Status: StatusOK}, {Status: StatusWarn}}
	fail := []Outcome{{Status: StatusWarn}, {Status: StatusFail}}

	assert.False(t, Failed(ok, false))
	assert.False(t, Failed(ok, true))
	assert.False(t, Failed(warn, false), "warnings are fine by default")
	assert.True(t, Failed(warn, true), "--strict makes a warning fail the run")
	assert.True(t, Failed(fail, false))
	assert.False(t, Failed(nil, true))
}

func TestStatusStringUnknown(t *testing.T) {
	assert.Equal(t, "Status(9)", Status(9).String())
}

func TestRunNamedDiamondRunsEachCheckOnce(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "a", ran: &ran},
		fakeCheck{name: "b", needs: []string{"a"}, ran: &ran},
		fakeCheck{name: "c", needs: []string{"a"}, ran: &ran},
		fakeCheck{name: "d", needs: []string{"b", "c"}, ran: &ran},
	}

	got, err := Run(context.Background(), checks, Options{Names: []string{"d"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c", "d"}, names(got))
	assert.Equal(t, []string{"a", "b", "c", "d"}, ran)
}

func TestRunStopsWhenTheCallerCancelsMidCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	checks := []Check{
		fakeCheck{name: "stuck", run: func(context.Context) Result { cancel(); <-block; return Result{} }},
	}

	got, err := Run(ctx, checks, Options{})

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, got)
}

func TestRunWithACancelledContextRunsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var ran []string

	got, err := Run(ctx, []Check{fakeCheck{name: "a", ran: &ran}}, Options{})

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, ran)
	assert.Empty(t, got)
}
