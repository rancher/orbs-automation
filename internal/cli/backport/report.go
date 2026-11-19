package backport

import (
	"errors"
	"fmt"
	"strings"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/cli/proposer"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrFailed is returned, with the report still written, when a target failed, was abandoned at a
// conflict, or was not attempted because an earlier one was.
var ErrFailed = errors.New("some targets were not backported")

// renderText is the report of a backport: what the PR is, what is picked, and for each target
// what was done, with the texts of the PR when full.
func renderText(p domain.Plan, full bool, view *style.Renderer) string {
	var b strings.Builder
	b.WriteString(view.Style(style.Heading, fmt.Sprintf("Backport: %s#%d", p.Repo, p.PR)) + " " + p.Title + "\n")
	if len(p.Picks) > 0 {
		short := make([]string, len(p.Picks))
		for i, c := range p.Picks {
			short[i] = c[:min(7, len(c))]
		}
		b.WriteString("  " + view.Style(style.Dim, fmt.Sprintf("merged to %s as a %s: picks %s", p.Base, p.Shape, strings.Join(short, ", "))) + "\n")
	}
	for _, tg := range p.Targets {
		label := tg.Line
		if label == "" {
			label = tg.Branch
		}
		b.WriteString("\n" + view.Style(style.Heading, label) + " " + view.Style(style.Dim, tg.Branch) + "\n")
		if tg.Preview != nil {
			b.WriteString("  branch  " + tg.Preview.Branch + "\n  title   " + tg.Preview.Title + "\n")
			if full {
				b.WriteString("\n" + indent(tg.Preview.Body) + "\n")
			}
		}
		if len(tg.Files) > 0 {
			b.WriteString("  resolved by hand: " + strings.Join(tg.Files, ", ") + "\n")
		}
		if r := tg.Result; r != nil {
			b.WriteString(resultText(view, tg, r))
			continue
		}
		b.WriteString("  " + view.Style(statusRole(tg.Status), string(tg.Status)) + "\n")
	}
	return b.String()
}

// resultText is what a run did for a target that was worked on.
func resultText(view *style.Renderer, tg domain.Target, r *proposal.TargetResult) string {
	var b strings.Builder
	status := string(r.Status)
	role := proposer.ResultRole(r.Status)
	if tg.Status == domain.StatusAborted {
		status, role = string(tg.Status), style.Error
	}
	b.WriteString("  " + view.Style(role, status))
	if r.PR != nil {
		b.WriteString(fmt.Sprintf(" #%d %s", r.PR.Number, r.PR.URL))
	}
	b.WriteString("\n")
	if r.Error != "" {
		b.WriteString("  " + view.Style(style.Error, r.Error) + "\n")
	}
	for _, c := range r.Commits {
		b.WriteString("  commit " + c.Hash[:min(7, len(c.Hash))] + " " + c.Subject + "\n")
	}
	for _, sup := range r.Superseded {
		b.WriteString(fmt.Sprintf("  closed #%d, replaced by this\n", sup.Number))
	}
	for _, warning := range r.Warnings {
		b.WriteString("  " + view.Style(style.Warn, warning) + "\n")
	}
	return b.String()
}

func statusRole(s domain.Status) style.Role {
	switch s {
	case domain.StatusAlreadyBackported:
		return style.OK
	case domain.StatusAborted:
		return style.Error
	case domain.StatusNotAttempted:
		return style.Warn
	}
	return style.Dim
}

// indent puts a block of text under its heading.
func indent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// failed is ErrFailed when a target did not get what it was asked for.
func failed(p domain.Plan) error {
	for _, tg := range p.Targets {
		if tg.Status == domain.StatusAborted || tg.Status == domain.StatusNotAttempted {
			return ErrFailed
		}
		if r := tg.Result; r != nil && (r.Status == proposal.ResultFailed || r.Status == proposal.ResultBlocked) {
			return ErrFailed
		}
	}
	return nil
}
