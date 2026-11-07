package ship

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

// styledPlans are plans that show the sections of the report: the check, the
// images, the PR, and each kind of result.
func styledPlans() map[string]domain.Plan {
	withNoPush := func(p domain.Plan) domain.Plan {
		p.NoPush, p.Clone, p.Remote = true, "/src/rancher", "upstream"
		return p
	}
	return map[string]domain.Plan{
		"preview":      previewPlan(),
		"with targets": previewPlanWithTargets(),
		"on lines":     onLines(),
		"each line":    eachLine(),
		"acting":       actingPlan(true, "bot"),
		"committed": withResults(withNoPush(previewPlan()),
			committed("bot/scc-operator-v0.5.3-v2.15", "abcdef0123456789", domain.InPlace, "")),
		"failed": withResults(previewPlan(),
			&domain.TargetResult{Status: domain.ResultFailed, Error: "boom"}),
		"blocked": withResults(previewPlan(),
			&domain.TargetResult{Status: domain.ResultBlocked, Error: "newer PR"}),
	}
}

// Styling adds escapes and changes nothing else, including where the columns of
// a table fall: the text is the plain text.
func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	plans := styledPlans()
	require.NotEmpty(t, plans)
	for name, p := range plans {
		plain := renderText(p, true)

		styled := renderTextStyled(p, true, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name+": something is styled")
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, p := range styledPlans() {
		plain := renderText(p, true)

		for who, st := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			got := renderTextStyled(p, true, st)

			assert.Equal(t, plain, got, name+" "+who)
			assert.NotContains(t, got, "\x1b", name+" "+who)
		}
	}
}

func TestSectionHeadingsAreHeadings(t *testing.T) {
	st := colored()

	out := renderTextStyled(styledPlans()["committed"], false, st)

	for _, heading := range []string{"Targets", "Images", "PR", "Results", "Review:"} {
		assert.Contains(t, out, st.Style(style.Heading, heading), heading)
	}
}

func TestResultsAndTargetsTakeTheColorOfTheirStatus(t *testing.T) {
	st := colored()
	committedOut := renderTextStyled(styledPlans()["committed"], false, st)
	failedOut := renderTextStyled(styledPlans()["failed"], false, st)
	blockedOut := renderTextStyled(styledPlans()["blocked"], false, st)

	assert.Contains(t, committedOut, st.Style(style.OK, "committed"))
	assert.Contains(t, failedOut, st.Style(style.Error, "failed"))
	assert.Contains(t, blockedOut, st.Style(style.Warn, "blocked"))
}

func TestWarningsAndFailuresStandOut(t *testing.T) {
	st := colored()
	p := styledPlans()["failed"]
	p.Targets[0].Warnings = []string{"this is a downgrade"}

	out := renderTextStyled(p, false, st)

	assert.Contains(t, out, st.Style(style.Warn, "Warning (release/v2.15): this is a downgrade"))
}

func TestTheJSONReportIsNeverStyled(t *testing.T) {
	run := func(st *style.Renderer) string {
		var out bytes.Buffer
		err := runToStyled(context.Background(), &out, nil, readyWorkspace(),
			fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputJSON, in, false, st)
		require.NoError(t, err)
		return out.String()
	}

	assert.Equal(t, run(style.Plain()), run(colored()))
	assert.NotContains(t, run(colored()), "\x1b")
}

func TestARunWritesTheStyledTextAndItsPlainTextIsTheSame(t *testing.T) {
	run := func(st *style.Renderer) string {
		var out bytes.Buffer
		err := runToStyled(context.Background(), &out, nil, readyWorkspace(),
			fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputText, in, false, st)
		require.NoError(t, err)
		return out.String()
	}

	plain, styled := run(style.Plain()), run(colored())

	assert.Contains(t, styled, "\x1b[")
	assert.Equal(t, plain, strip(styled))
}
