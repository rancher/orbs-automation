package doctor

import (
	"encoding/json"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBuild = Build{Version: "0.4.0", Commit: "abc1234", Date: "2026-10-29", OS: "linux", Arch: "arm64"}

// mixedOutcomes has every status, in registry order, with a category that
// comes back after another one.
func mixedOutcomes() []Outcome {
	return []Outcome{
		{Name: "version", Category: "environment", Status: StatusOK, Message: "gero 0.4.0 on linux/arm64"},
		{Name: "git", Category: "dependencies", Status: StatusOK, Message: "2.51.0, user.name and user.email are set"},
		{Name: "gh", Category: "dependencies", Status: StatusWarn, Message: "not found", Remediation: "install gh, or set GH_TOKEN"},
		{Name: "config", Category: "configuration", Status: StatusFail, Message: "2 problems",
			Remediation: "run `gero config validate` for the details"},
		{Name: "repos", Category: "configuration", Status: StatusSkip, SkipReason: "needs config"},
		{Name: "auth-token", Category: "authentication", Status: StatusOK, Message: "environment variable GH_TOKEN"},
		{Name: "auth-api", Category: "authentication", Online: true, Status: StatusSkip, SkipReason: "run with --online"},
	}
}

func TestRenderText(t *testing.T) {
	got, err := Render(Report{Build: testBuild, Outcomes: mixedOutcomes()}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Equal(t, `gero 0.4.0 (commit abc1234, built 2026-10-29)

Environment
  ✔ version: gero 0.4.0 on linux/arm64

Dependencies
  ✔ git: 2.51.0, user.name and user.email are set
  ⚠ gh: not found
    → install gh, or set GH_TOKEN

Configuration
  ✗ config: 2 problems
    → run `+"`gero config validate`"+` for the details
  ○ repos: skipped, needs config

Authentication
  ✔ auth-token: environment variable GH_TOKEN
  ○ auth-api: skipped, run with --online

Summary: 3 ok, 1 warning, 1 failed, 2 skipped
1 check needs --online. Some checks failed: fix the errors above, then run gero doctor again.
`, got)
}

func TestRenderTextVerdicts(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []Outcome
		want     string
	}{
		{
			name:     "all ok",
			outcomes: []Outcome{{Name: "git", Category: "dependencies", Status: StatusOK, Message: "fine"}},
			want: "Summary: 1 ok, 0 warnings, 0 failed, 0 skipped\n" +
				"Your workstation is ready to use gero.\n",
		},
		{
			name: "warnings only, and checks that need --online",
			outcomes: []Outcome{
				{Name: "gh", Category: "dependencies", Status: StatusWarn, Message: "not found", Remediation: "install it"},
				{Name: "gh2", Category: "dependencies", Status: StatusWarn, Message: "old", Remediation: "update it"},
				{Name: "a", Category: "authentication", Online: true, Status: StatusSkip, SkipReason: "run with --online"},
				{Name: "b", Category: "authentication", Online: true, Status: StatusSkip, SkipReason: "run with --online"},
			},
			want: "Summary: 0 ok, 2 warnings, 0 failed, 2 skipped\n" +
				"2 checks need --online. Ready, with warnings.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(Report{Build: testBuild, Outcomes: tt.outcomes}, clicfg.OutputText)

			require.NoError(t, err)
			assert.Contains(t, got, tt.want)
			assert.Regexp(t, `(?s)\n\n`+tt.want[:len("Summary")]+`.*\z`, got, "the summary follows a blank line and ends the report")
		})
	}
}

func TestRenderTextUnknownCategoriesComeLastInTheOrderSeen(t *testing.T) {
	got, err := Render(Report{Build: testBuild, Outcomes: []Outcome{
		{Name: "z", Category: "zebra", Status: StatusOK, Message: "ok"},
		{Name: "g", Category: "dependencies", Status: StatusOK, Message: "ok"},
		{Name: "m", Category: "mongoose", Status: StatusOK, Message: "ok"},
	}}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?s)Dependencies.*Zebra.*Mongoose`, got)
}

func TestRenderTextCheckWithoutACategoryGoesUnderOther(t *testing.T) {
	got, err := Render(Report{Build: testBuild, Outcomes: []Outcome{
		{Name: "g", Category: "dependencies", Status: StatusOK, Message: "ok"},
		{Name: "x", Status: StatusOK, Message: "ok"},
	}}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Contains(t, got, "\nOther\n  ✔ x: ok\n")
	assert.Regexp(t, `(?s)Dependencies.*Other`, got)
}

func TestRenderJSON(t *testing.T) {
	got, err := Render(Report{Build: testBuild, Outcomes: mixedOutcomes()}, clicfg.OutputJSON)

	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Equal(t, byte('\n'), got[len(got)-1], "ends with a newline")
	var decoded struct {
		Gero    Build `json:"gero"`
		Checks  []map[string]any
		Summary map[string]int
	}
	require.NoError(t, json.Unmarshal([]byte(got), &decoded))
	assert.Equal(t, testBuild, decoded.Gero)
	assert.Equal(t, map[string]int{"ok": 3, "warn": 1, "fail": 1, "skip": 2}, decoded.Summary)
	require.Len(t, decoded.Checks, 7)
	assert.Equal(t, map[string]any{
		"name": "gh", "category": "dependencies", "online": false, "status": "warn",
		"message": "not found", "remediation": "install gh, or set GH_TOKEN",
	}, decoded.Checks[2], "skipReason is left out unless the check was skipped")
	assert.Equal(t, map[string]any{
		"name": "auth-api", "category": "authentication", "online": true, "status": "skip",
		"message": "", "remediation": "", "skipReason": "run with --online",
	}, decoded.Checks[6])
}

func TestRenderJSONWithNoChecksHasAnEmptyList(t *testing.T) {
	got, err := Render(Report{Build: testBuild}, clicfg.OutputJSON)

	require.NoError(t, err)
	assert.Contains(t, got, `"checks": []`)
}

func TestRenderUnsupportedFormats(t *testing.T) {
	for _, format := range []string{clicfg.OutputEnv, "yaml"} {
		t.Run(format, func(t *testing.T) {
			got, err := Render(Report{Build: testBuild, Outcomes: mixedOutcomes()}, format)

			require.Error(t, err)
			assert.ErrorContains(t, err, format)
			assert.Empty(t, got)
		})
	}
}

func TestRenderChecksText(t *testing.T) {
	checks := []Check{
		fakeCheck{name: "git", category: "dependencies"},
		fakeCheck{name: "auth-api", category: "authentication", online: true},
	}

	got, err := RenderChecks(checks, clicfg.OutputText)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"NAME      CATEGORY        ONLINE  DESCRIPTION\n"+
		"git       dependencies    no      checks git\n"+
		"auth-api  authentication  yes     checks auth-api\n", got)
}

func TestRenderChecksJSON(t *testing.T) {
	checks := []Check{
		fakeCheck{name: "git", category: "dependencies"},
		fakeCheck{name: "auth-api", category: "authentication", online: true},
	}

	got, err := RenderChecks(checks, clicfg.OutputJSON)

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"name":"git","category":"dependencies","description":"checks git","online":false},
		{"name":"auth-api","category":"authentication","description":"checks auth-api","online":true}
	]`, got)
}

func TestRenderChecksUnsupportedFormat(t *testing.T) {
	got, err := RenderChecks([]Check{fakeCheck{name: "git"}}, clicfg.OutputEnv)

	require.Error(t, err)
	assert.Empty(t, got)
}
