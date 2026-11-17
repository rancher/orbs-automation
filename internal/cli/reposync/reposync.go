// Package reposync holds the logic of gero repo sync: what the command asks of the
// reposync domain and how it reports what came of it. It is framework-free: cmd/ owns
// the cobra wiring.
package reposync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/rancher/orbs-automation/internal/cli/proposer"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/proposal"
	domain "github.com/rancher/orbs-automation/internal/reposync"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrFailed is returned, with the report still written, when a row could not be read or a
// branch failed.
var ErrFailed = errors.New("some branches failed")

// Options is what the user asked to sync; the command fills it from its
// arguments and flags.
type Options struct {
	Project  string   // the holder, by config name or slug
	Holdings []string // limit the run to these holdings
	Lines    []string // limit the run to these Rancher lines
	Preview  bool     // print the commit message and PR body too
	NoPush   bool
	PushTo   string
	Yes      bool
	// Interactive says In is a terminal the question can be asked on.
	Interactive bool
	In          io.Reader
}

// Run brings what a project holds in line with config: it checks each branch, makes
// the commits for the corrections an Edit follows and, unless --no-push or --dry-run,
// pushes each branch and opens its PR after asking once. The report is written to w, as text or as one
// document in json, and it is written even when the error returned is ErrFailed or
// ErrDeclined.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts Options) error {
	if err := checkFormat(s.Output); err != nil {
		return err
	}
	ctx, ex, cleanup, err := proposer.Build(ctx, w, log, s, proposer.Options{NoPush: opts.NoPush, Yes: opts.Yes, Interactive: opts.Interactive, In: opts.In})
	if err != nil {
		return err
	}
	defer cleanup()
	in := domain.Input{
		Project: opts.Project, Holdings: opts.Holdings, Lines: opts.Lines, BranchPrefix: s.BranchPrefix,
		DryRun: s.DryRun, NoPush: opts.NoPush, PushTo: opts.PushTo, CI: s.CI,
	}
	return runTo(ctx, w, proposer.Workspace{Workspace: workspace.Open(s, cwd)}, ex, s.Output, in, opts.Preview, style.New(s.Terminal))
}

// runTo checks and fixes, with an executor, and writes the report. Errors that
// come after the check (no clone, a failed target, declined) are returned after it.
func runTo(ctx context.Context, w io.Writer, ws domain.Workspace, ex *proposal.Executor, format string, in domain.Input, full bool, st *style.Renderer) error {
	if err := checkFormat(format); err != nil {
		return err
	}
	plan, err := domain.Run(ctx, ws, in, ex)
	if errors.Is(err, workspace.ErrNotProject) {
		return fmt.Errorf("%w: name the project to sync as the first argument", err)
	}
	if err != nil && len(plan.Branches) == 0 {
		return err // nothing was found, so there is no report: "nothing to sync" would be untrue
	}
	out := renderSyncText(plan, full, st)
	if format == clicfg.OutputJSON {
		b, jerr := json.MarshalIndent(plan, "", "  ")
		if jerr != nil {
			return jerr
		}
		out = string(b) + "\n"
	}
	if _, werr := io.WriteString(w, out); werr != nil {
		return werr
	}
	if err != nil {
		return err
	}
	return syncFailed(plan)
}

// syncFailed is ErrFailed when a row could not be read or a branch failed, whether its
// commit or its push did: the exit code says the run did not do everything it was asked.
func syncFailed(p domain.Plan) error {
	for _, b := range p.Branches {
		if b.Status == domain.StatusFailed || (b.Result != nil && (b.Result.Status == proposal.ResultFailed || b.Result.Status == proposal.ResultBlocked)) {
			return ErrFailed
		}
		for _, r := range b.Rows {
			if r.Status == domain.StatusFailed {
				return ErrFailed
			}
		}
	}
	return nil
}

// renderSyncText is the report of a sync: a row for each holding on each line, and for
// each branch that was worked on what was done, with the texts of the PR when full.
func renderSyncText(p domain.Plan, full bool, view *style.Renderer) string {
	var b strings.Builder
	b.WriteString(view.Style(style.Heading, "Sync: "+p.Holder) + "\n")
	if len(p.Branches) == 0 {
		b.WriteString("  " + view.Style(style.Dim, "nothing to sync: no holding that config brings in line") + "\n")
		return b.String()
	}
	table := [][]string{{"  " + view.Style(style.Dim, "LINE"), view.Style(style.Dim, "BRANCH"), view.Style(style.Dim, "HOLDING"),
		view.Style(style.Dim, "HOLDS"), view.Style(style.Dim, "WANTS"), view.Style(style.Dim, "STATUS")}}
	for _, br := range p.Branches {
		for _, r := range br.Rows {
			table = append(table, []string{"  " + r.Line, orDash(r.Branch), r.Holding, orDash(r.Current), orDash(r.Want), syncStatus(view, r)})
		}
	}
	b.WriteString(ui.Align(table, view.Width))
	for _, br := range p.Branches {
		if br.Result == nil && !(full && br.Preview != nil) {
			continue
		}
		b.WriteString("\n" + view.Style(style.Heading, br.Branch) + "\n")
		if br.Preview != nil {
			b.WriteString("  branch  " + br.Preview.Branch + "\n  title   " + br.Preview.Title + "\n")
			if full {
				b.WriteString("\n" + indent(br.Preview.CommitMessage) + "\n" + indent(br.Preview.Body) + "\n")
			}
		}
		if r := br.Result; r != nil {
			b.WriteString("  " + view.Style(proposer.ResultRole(r.Status), string(r.Status)))
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
		}
	}
	return b.String()
}

// syncStatus is how one row stands, in a few words.
func syncStatus(view *style.Renderer, r domain.Row) string {
	switch r.Status {
	case domain.StatusUpToDate:
		return view.Style(style.OK, "current")
	case domain.StatusReady:
		return view.Style(style.Warn, "differs, ready")
	case domain.StatusBlocked:
		return view.Style(style.Warn, "differs, left alone: "+r.Reason)
	}
	return view.Style(style.Error, "unreadable: "+r.Reason)
}

func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

// indent puts a block of text under its heading.
func indent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// checkFormat refuses an output format that the command does not write.
func checkFormat(format string) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by repo sync: use text or json", format)
	}
	return nil
}
