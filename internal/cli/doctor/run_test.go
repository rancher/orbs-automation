package doctor

import (
	"context"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func settings(format string) clicfg.Settings { return clicfg.Settings{Output: format} }

func TestRunReportsAPassingRun(t *testing.T) {
	var ran []string
	checks := []Check{fakeCheck{name: "git", category: "dependencies", ran: &ran}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{}, testBuild, checks)

	require.NoError(t, err)
	assert.Equal(t, []string{"git"}, ran)
	assert.Contains(t, out, "gero 0.4.0 (commit abc1234, built 2026-10-29)\n")
	assert.Contains(t, out, "✔ git: git is fine\n")
	assert.Contains(t, out, "Your workstation is ready to use gero.\n")
}

func TestRunJSONFormat(t *testing.T) {
	checks := []Check{fakeCheck{name: "git", category: "dependencies"}}

	out, err := run(context.Background(), settings(clicfg.OutputJSON), Flags{}, testBuild, checks)

	require.NoError(t, err)
	assert.Contains(t, out, `"name": "git"`)
}

func TestRunAFailureReturnsTheReportAndTheError(t *testing.T) {
	checks := []Check{fakeCheck{name: "git", category: "dependencies", run: result(StatusFail)}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{}, testBuild, checks)

	require.ErrorIs(t, err, ErrChecksFailed)
	assert.Contains(t, out, "✗ git: msg\n", "the report is printed even though the run failed")
}

func TestRunStrictTurnsAWarningIntoAFailure(t *testing.T) {
	checks := []Check{fakeCheck{name: "gh", category: "dependencies", run: result(StatusWarn)}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{}, testBuild, checks)
	require.NoError(t, err, "a warning is fine by default")
	assert.Contains(t, out, "⚠ gh")

	out, err = run(context.Background(), settings(clicfg.OutputText), Flags{Strict: true}, testBuild, checks)
	require.ErrorIs(t, err, ErrChecksFailed)
	assert.Contains(t, out, "⚠ gh")
}

func TestRunListChecksDoesNotRunThem(t *testing.T) {
	var ran []string
	checks := []Check{
		fakeCheck{name: "git", category: "dependencies", ran: &ran},
		fakeCheck{name: "api", category: "authentication", online: true, ran: &ran},
	}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{ListChecks: true}, testBuild, checks)

	require.NoError(t, err)
	assert.Empty(t, ran)
	assert.Contains(t, out, "NAME  CATEGORY        ONLINE  DESCRIPTION\n")
	assert.Contains(t, out, "git   dependencies    no      checks git\n")
	assert.Contains(t, out, "api   authentication  yes     checks api\n")
}

func TestRunPassesTheFlagsToTheRunner(t *testing.T) {
	var ran []string
	var deadline time.Time
	checks := []Check{
		fakeCheck{name: "git", ran: &ran},
		fakeCheck{name: "api", online: true, ran: &ran, run: func(ctx context.Context) Result {
			deadline, _ = ctx.Deadline()
			return Result{Status: StatusOK, Message: "up"}
		}},
		fakeCheck{name: "other", ran: &ran},
	}

	out, err := run(context.Background(), settings(clicfg.OutputText),
		Flags{Names: []string{"api"}, Timeout: 3 * time.Second}, testBuild, checks)

	require.NoError(t, err)
	assert.Equal(t, []string{"api"}, ran, "only the named check, and naming an online one needs no --online")
	assert.InDelta(t, 3, time.Until(deadline).Seconds(), 1)
	assert.Contains(t, out, "✔ api: up")
	assert.NotContains(t, out, "git")

	ran = nil
	out, err = run(context.Background(), settings(clicfg.OutputText), Flags{Online: true}, testBuild, checks)
	require.NoError(t, err)
	assert.Equal(t, []string{"git", "api", "other"}, ran)
	assert.NotContains(t, out, "--online")
}

func TestRunOfflineSaysHowManyChecksNeedOnline(t *testing.T) {
	checks := []Check{fakeCheck{name: "git"}, fakeCheck{name: "api", online: true}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{}, testBuild, checks)

	require.NoError(t, err)
	assert.Contains(t, out, "○ api: skipped, run with --online\n")
	assert.Contains(t, out, "1 check needs --online. ")
}

func TestRunUnknownCheckPointsAtTheList(t *testing.T) {
	var ran []string
	checks := []Check{fakeCheck{name: "git", ran: &ran}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{Names: []string{"gti"}}, testBuild, checks)

	require.ErrorIs(t, err, ErrUnknownCheck)
	assert.ErrorContains(t, err, "gti")
	assert.ErrorContains(t, err, "--list-checks")
	assert.Empty(t, out)
	assert.Empty(t, ran)
}

func TestRunUnsupportedFormatFailsBeforeRunningAnything(t *testing.T) {
	var ran []string
	checks := []Check{fakeCheck{name: "git", ran: &ran}}

	for _, f := range []Flags{{}, {ListChecks: true}} {
		out, err := run(context.Background(), settings(clicfg.OutputEnv), f, testBuild, checks)

		require.Error(t, err)
		assert.ErrorContains(t, err, "env")
		assert.Empty(t, out)
		assert.Empty(t, ran, "no check should run for a format that cannot be rendered")
	}
}

func TestRunAnInconsistentRegistryIsAnError(t *testing.T) {
	checks := []Check{fakeCheck{name: "a"}, fakeCheck{name: "a"}}

	out, err := run(context.Background(), settings(clicfg.OutputText), Flags{}, testBuild, checks)

	require.ErrorIs(t, err, ErrBadRegistry)
	assert.Empty(t, out)
}

func TestMainRunsTheDefaultChecks(t *testing.T) {
	out, err := Main(context.Background(), settings(clicfg.OutputText), Flags{Names: []string{"version"}}, testBuild)

	require.NoError(t, err)
	assert.Contains(t, out, "✔ version: gero 0.4.0 on linux/arm64\n")
}
