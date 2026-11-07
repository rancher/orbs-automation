package doctor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// Build says which gero produced a report, for the header and the JSON.
type Build struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Report is everything the renderers need.
type Report struct {
	Build    Build
	Outcomes []Outcome
	// Style colors the text format. Nil, and one built for a terminal without
	// color, leave it plain; the JSON format is never styled.
	Style *style.Renderer
}

// categoryOrder is the order of the known groups in the text output. A category
// not listed comes after them, in the order it first appears.
var categoryOrder = []string{"environment", "dependencies", "configuration", "authentication"}

// markers keep the text readable without color.
var markers = map[Status]string{
	StatusOK:   "✔",
	StatusWarn: "⚠",
	StatusFail: "✗",
	StatusSkip: "○",
}

func unsupported(format string) error {
	return fmt.Errorf("gero doctor does not support output format %q (use text or json)", format)
}

// Render formats r as clicfg.OutputText or clicfg.OutputJSON. A check's message
// is whatever the check chose to say, and no check is given a token to say it
// with.
func Render(r Report, format string) (string, error) {
	switch format {
	case clicfg.OutputText:
		return renderText(r), nil
	case clicfg.OutputJSON:
		return renderJSON(r)
	}
	return "", unsupported(format)
}

// styled is text in the look of role, or the text itself when the report has no
// renderer or the renderer has no color.
func (r Report) styled(role style.Role, text string) string {
	if r.Style == nil {
		return text
	}
	return r.Style.Style(role, text)
}

// markerRoles are the looks of the markers: a pass, a warning, a failure, and a
// skip that is not one of them.
var markerRoles = map[Status]style.Role{
	StatusOK:   style.OK,
	StatusWarn: style.Warn,
	StatusFail: style.Error,
	StatusSkip: style.Dim,
}

func renderText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", r.styled(style.Dim, fmt.Sprintf("gero %s (commit %s, built %s)", r.Build.Version, r.Build.Commit, r.Build.Date)))
	for _, category := range orderedCategories(r.Outcomes) {
		fmt.Fprintf(&b, "\n%s\n", r.styled(style.Heading, title(category)))
		for _, o := range r.Outcomes {
			if o.Category != category {
				continue
			}
			msg := o.Message
			if o.Status == StatusSkip {
				msg = "skipped, " + o.SkipReason
			}
			fmt.Fprintf(&b, "  %s %s: %s\n", r.styled(markerRoles[o.Status], markers[o.Status]), o.Name, msg)
			if o.Remediation != "" {
				fmt.Fprintf(&b, "    %s\n", r.styled(style.Dim, "→ "+o.Remediation))
			}
		}
	}
	n := Summarize(r.Outcomes)
	fmt.Fprintf(&b, "\nSummary: %d ok, %d %s, %d failed, %d skipped\n", n.OK, n.Warn, plural(n.Warn, "warning"), n.Fail, n.Skip)
	switch online := NeedOnline(r.Outcomes); {
	case online == 1:
		b.WriteString("1 check needs --online. ")
	case online > 1:
		fmt.Fprintf(&b, "%d checks need --online. ", online)
	}
	switch {
	case n.Fail > 0:
		b.WriteString(r.styled(style.Error, "Some checks failed: fix the errors above, then run gero doctor again.") + "\n")
	case n.Warn > 0:
		b.WriteString(r.styled(style.Warn, "Ready, with warnings.") + "\n")
	default:
		b.WriteString(r.styled(style.OK, "Your workstation is ready to use gero.") + "\n")
	}
	return b.String()
}

// title is the heading of a category group. A check with no category lands in
// "Other" rather than breaking the report.
func title(category string) string {
	if category == "" {
		return "Other"
	}
	return strings.ToUpper(category[:1]) + category[1:]
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// orderedCategories lists the categories present in outcomes: the known ones in
// categoryOrder, then the others in the order they first appear.
func orderedCategories(outcomes []Outcome) []string {
	present := map[string]bool{}
	var seen []string
	for _, o := range outcomes {
		if !present[o.Category] {
			present[o.Category] = true
			seen = append(seen, o.Category)
		}
	}
	var out []string
	known := map[string]bool{}
	for _, c := range categoryOrder {
		known[c] = true
		if present[c] {
			out = append(out, c)
		}
	}
	for _, c := range seen {
		if !known[c] {
			out = append(out, c)
		}
	}
	return out
}

func renderJSON(r Report) (string, error) {
	type check struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Online      bool   `json:"online"`
		Status      string `json:"status"`
		Message     string `json:"message"`
		Remediation string `json:"remediation"`
		SkipReason  string `json:"skipReason,omitempty"`
	}
	type summary struct {
		OK   int `json:"ok"`
		Warn int `json:"warn"`
		Fail int `json:"fail"`
		Skip int `json:"skip"`
	}
	checks := make([]check, 0, len(r.Outcomes)) // an empty list, never null
	for _, o := range r.Outcomes {
		checks = append(checks, check{o.Name, o.Category, o.Online, o.Status.String(), o.Message, o.Remediation, o.SkipReason})
	}
	n := Summarize(r.Outcomes)
	return marshal(struct {
		Gero    Build   `json:"gero"`
		Checks  []check `json:"checks"`
		Summary summary `json:"summary"`
	}{r.Build, checks, summary{n.OK, n.Warn, n.Fail, n.Skip}})
}

// RenderChecks formats the check list for --list-checks as clicfg.OutputText or
// clicfg.OutputJSON.
func RenderChecks(checks []Check, format string, view *style.Renderer) (string, error) {
	switch format {
	case clicfg.OutputText:
		rows := [][]string{{
			view.Style(style.Dim, "NAME"), view.Style(style.Dim, "CATEGORY"),
			view.Style(style.Dim, "ONLINE"), view.Style(style.Dim, "DESCRIPTION"),
		}}
		for _, c := range checks {
			online := view.Style(style.Dim, "no")
			if c.Online() {
				online = "yes"
			}
			rows = append(rows, []string{c.Name(), view.Style(style.Dim, c.Category()), online, c.Description()})
		}
		return ui.Align(rows, view.Width), nil
	case clicfg.OutputJSON:
		type entry struct {
			Name        string `json:"name"`
			Category    string `json:"category"`
			Description string `json:"description"`
			Online      bool   `json:"online"`
		}
		list := make([]entry, 0, len(checks))
		for _, c := range checks {
			list = append(list, entry{c.Name(), c.Category(), c.Description(), c.Online()})
		}
		return marshal(list)
	}
	return "", unsupported(format)
}

func marshal(v any) (string, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
