package ship

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui"
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

	for _, heading := range []string{"Targets", "PR", "Results", "Review:"} {
		assert.Contains(t, out, st.Style(style.Heading, heading), heading)
	}
}

func TestARunStylesTheImagesSectionAndItsStatuses(t *testing.T) {
	st := colored()
	var out bytes.Buffer

	err := runToStyled(context.Background(), &out, nil, readyWorkspace(),
		fakeProber{result: registry.Result{Status: registry.Found}}, nil, clicfg.OutputText, in, false, st)

	require.NoError(t, err)
	assert.Contains(t, out.String(), st.Style(style.Heading, "Images"))
	assert.Contains(t, out.String(), st.Style(style.OK, "found"))
	assert.Contains(t, out.String(), st.Style(style.Dim, "IMAGE"), "column heads are secondary text")
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

func TestTheQuestionIsStyledAndItsPlainTextIsTheSame(t *testing.T) {
	st := colored()
	q := question(opening("main"))
	text := strings.TrimSuffix(q, " [y/N] ")
	var out bytes.Buffer

	ok, err := confirmer(strings.NewReader("y\n"), &out, true, false, st)(context.Background(), opening("main"))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, out.String(), st.Style(style.Heading, text), "the question is what the user is to read")
	assert.Contains(t, out.String(), st.Style(style.Dim, "[y/N]"), "the choices are secondary text")
	assert.Equal(t, "\n"+q, strip(out.String()))
}

func TestTheQuestionWithoutColorIsThePlainQuestion(t *testing.T) {
	for who, st := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		var out bytes.Buffer

		_, err := confirmer(strings.NewReader("n\n"), &out, true, false, st)(context.Background(), opening("main"))

		require.NoError(t, err, who)
		assert.Equal(t, "\n"+question(opening("main")), out.String(), who)
	}
}

func TestYesNeverAsksSoNothingIsStyled(t *testing.T) {
	var out bytes.Buffer

	_, err := confirmer(strings.NewReader(""), &out, false, true, colored())(context.Background(), opening("main"))

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

// event is the text one event adds to the report.
func event(t *testing.T, st *style.Renderer, e domain.Event) string {
	t.Helper()
	var out bytes.Buffer
	stream := ui.NewStream(&out, ui.WithWidth(st.Width))
	newStyledSink(stream, false, st).Event(e)
	stream.Flush()
	return out.String()
}

func TestTheLinesOfWhatWasDoneNameTheirVerbInTheLookOfIt(t *testing.T) {
	st := colored()
	pr := &domain.PRRef{Number: 7, URL: "https://github.com/rancher/rancher/pull/7"}
	rows := []struct {
		name string
		e    domain.Event
		verb string
		role style.Role
		line string
	}{
		{"pushed", domain.Event{Kind: domain.EventPushed, Line: "v2.15", NewBranch: "bot/x", Remote: "origin"}, "pushed", style.OK,
			"  v2.15: pushed bot/x to origin\n"},
		{"opened", domain.Event{Kind: domain.EventPROpened, Line: "v2.15", PR: pr}, "opened", style.OK,
			"  v2.15: opened #7 https://github.com/rancher/rancher/pull/7\n"},
		{"refreshed", domain.Event{Kind: domain.EventRefreshed, Line: "v2.15", PR: pr}, "refreshed", style.OK,
			"  v2.15: refreshed #7 https://github.com/rancher/rancher/pull/7\n"},
		{"closed", domain.Event{Kind: domain.EventSuperseded, Line: "v2.15", PR: pr}, "closed", style.Dim,
			"  v2.15: closed #7\n"},
	}
	for _, r := range rows {
		got := event(t, st, r.e)

		assert.Equal(t, r.line, strip(got), r.name+": the text is the same")
		assert.Contains(t, got, st.Style(r.role, r.verb), r.name)
		assert.Equal(t, r.line, event(t, style.Plain(), r.e), r.name+": plain")
	}
}

func TestTheResultLinesTakeTheLookOfTheirStatus(t *testing.T) {
	st := colored()
	pr := &domain.PRRef{Number: 7, URL: "https://x/7"}
	rows := []struct {
		name   string
		result domain.TargetResult
		styled string
		role   style.Role
	}{
		{"already open", domain.TargetResult{Status: domain.ResultAlreadyOpen, PR: pr}, "already open", style.Dim},
		{"would refresh", domain.TargetResult{Status: domain.ResultWouldRefresh, PR: pr}, "would refresh", style.Dim},
		{"up to date", domain.TargetResult{Status: domain.ResultUpToDate}, "already up to date", style.Dim},
	}
	for _, r := range rows {
		res := r.result
		got := event(t, st, domain.Event{Kind: domain.EventTargetDone, Line: "v2.15", Branch: "main", Result: &res})

		assert.Equal(t, event(t, style.Plain(), domain.Event{Kind: domain.EventTargetDone, Line: "v2.15", Branch: "main", Result: &res}), strip(got), r.name)
		assert.Contains(t, got, st.Style(r.role, r.styled), r.name)
	}
}

func TestTheLabelsOfTheSummaryAreSecondaryText(t *testing.T) {
	st := colored()

	out := renderTextStyled(styledPlans()["acting"], false, st)

	for _, label := range []string{"method", "status", "acting as"} {
		assert.Contains(t, out, st.Style(style.Dim, label), label)
	}
}
