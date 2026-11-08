package doctor

import (
	"context"
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func render(t *testing.T, r Report, format string) string {
	t.Helper()
	got, err := Render(r, format)
	require.NoError(t, err)
	return got
}

func reports() map[string][]Outcome {
	return map[string][]Outcome{
		"mixed": mixedOutcomes(),
		"all ok": {
			{Name: "git", Category: "dependencies", Status: StatusOK, Message: "fine"},
		},
		"a warning": {
			{Name: "gh", Category: "dependencies", Status: StatusWarn, Message: "not found", Remediation: "install gh"},
		},
		"none": nil,
	}
}

// Styling adds escapes and changes nothing else: the text is the same text.
func TestStyledTextIsThePlainTextWithEscapesAdded(t *testing.T) {
	for name, outcomes := range reports() {
		plain := render(t, Report{Build: testBuild, Outcomes: outcomes}, clicfg.OutputText)

		styled := render(t, Report{Build: testBuild, Outcomes: outcomes, Style: colored()}, clicfg.OutputText)

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name+": something is colored")
	}
}

func TestNoColorAndNilAreThePlainText(t *testing.T) {
	plain := render(t, Report{Build: testBuild, Outcomes: mixedOutcomes()}, clicfg.OutputText)

	for name, st := range map[string]*style.Renderer{"plain renderer": style.Plain(), "profile none": style.New(clicfg.Terminal{})} {
		got := render(t, Report{Build: testBuild, Outcomes: mixedOutcomes(), Style: st}, clicfg.OutputText)

		assert.Equal(t, plain, got, name)
		assert.NotContains(t, got, "\x1b", name)
	}
}

func TestMarkersAndHeadingsTakeTheirRoles(t *testing.T) {
	st := colored()

	got := render(t, Report{Build: testBuild, Outcomes: mixedOutcomes(), Style: st}, clicfg.OutputText)

	assert.Contains(t, got, st.Style(style.OK, "✔"))
	assert.Contains(t, got, st.Style(style.Warn, "⚠"))
	assert.Contains(t, got, st.Style(style.Error, "✗"))
	assert.Contains(t, got, st.Style(style.Dim, "○"))
	assert.Contains(t, got, st.Style(style.Heading, "Dependencies"))
	assert.Contains(t, got, st.Style(style.Dim, "→ install gh, or set GH_TOKEN"), "a remediation is secondary text")
}

func TestTheClosingLineTakesTheColorOfTheOutcome(t *testing.T) {
	st := colored()
	closing := map[string]struct {
		role style.Role
		text string
	}{
		"mixed":     {style.Error, "Some checks failed: fix the errors above, then run gero doctor again."},
		"a warning": {style.Warn, "Ready, with warnings."},
		"all ok":    {style.OK, "Your workstation is ready to use gero."},
	}
	for name, want := range closing {
		got := render(t, Report{Build: testBuild, Outcomes: reports()[name], Style: st}, clicfg.OutputText)

		assert.Contains(t, got, st.Style(want.role, want.text), name)
	}
}

func TestJSONIsNeverStyled(t *testing.T) {
	plain := render(t, Report{Build: testBuild, Outcomes: mixedOutcomes()}, clicfg.OutputJSON)

	got := render(t, Report{Build: testBuild, Outcomes: mixedOutcomes(), Style: colored()}, clicfg.OutputJSON)

	assert.Equal(t, plain, got)
	assert.NotContains(t, got, "\x1b")
}

func TestRunStylesTheTextFromTheTerminalOfTheSettings(t *testing.T) {
	checks := []Check{fakeCheck{name: "git", category: "dependencies"}}
	s := clicfg.Settings{Output: clicfg.OutputText, Terminal: clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile16}}

	styled, err := run(context.Background(), s, Flags{}, testBuild, checks)
	require.NoError(t, err)
	s.Terminal = clicfg.Terminal{}
	plain, err := run(context.Background(), s, Flags{}, testBuild, checks)
	require.NoError(t, err)

	assert.Contains(t, styled, "\x1b[")
	assert.NotContains(t, plain, "\x1b")
	assert.Equal(t, plain, strip(styled))
}

func TestRunNeverStylesJSON(t *testing.T) {
	checks := []Check{fakeCheck{name: "git", category: "dependencies"}}
	s := clicfg.Settings{Output: clicfg.OutputJSON, Terminal: clicfg.Terminal{StdoutTTY: true, Profile: clicfg.ProfileTrueColor}}

	out, err := run(context.Background(), s, Flags{}, testBuild, checks)

	require.NoError(t, err)
	assert.NotContains(t, out, "\x1b")
}

func listed() []Check {
	return []Check{
		fakeCheck{name: "git", category: "dependencies"},
		fakeCheck{name: "auth-api", category: "authentication", online: true},
	}
}

func TestStyledCheckListIsThePlainListWithEscapesAdded(t *testing.T) {
	plain, err := RenderChecks(listed(), clicfg.OutputText, nil)
	require.NoError(t, err)

	styled, err := RenderChecks(listed(), clicfg.OutputText, colored())

	require.NoError(t, err)
	assert.Equal(t, plain, strip(styled))
	assert.NotEqual(t, plain, styled)
}

func TestNoColorIsThePlainCheckList(t *testing.T) {
	plain, err := RenderChecks(listed(), clicfg.OutputText, nil)
	require.NoError(t, err)

	for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		got, err := RenderChecks(listed(), clicfg.OutputText, v)

		require.NoError(t, err, who)
		assert.Equal(t, plain, got, who)
	}
	assert.NotContains(t, plain, "\x1b")
}

func TestTheCheckListHeadsAndCategoriesAreSecondaryText(t *testing.T) {
	v := colored()

	out, err := RenderChecks(listed(), clicfg.OutputText, v)

	require.NoError(t, err)
	for _, head := range []string{"NAME", "CATEGORY", "ONLINE", "DESCRIPTION"} {
		assert.Contains(t, out, v.Style(style.Dim, head), head)
	}
	assert.Contains(t, out, v.Style(style.Dim, "dependencies"))
	assert.Contains(t, out, v.Style(style.Dim, "no"), "a check that needs no network")
	assert.Contains(t, out, "yes", "a check that does is not secondary")
	assert.NotContains(t, out, v.Style(style.Dim, "yes"))
}

func TestTheJSONCheckListIsNeverStyled(t *testing.T) {
	plain, err := RenderChecks(listed(), clicfg.OutputJSON, nil)
	require.NoError(t, err)

	got, err := RenderChecks(listed(), clicfg.OutputJSON, colored())

	require.NoError(t, err)
	assert.Equal(t, plain, got)
	assert.NotContains(t, got, "\x1b")
}
