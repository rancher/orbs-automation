package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// SeriesOptions is what the user asked to fix; the command fills it from its
// arguments and flags.
type SeriesOptions struct {
	Project string   // the holder, by config name or slug
	Holding string   // limit the run to one holding
	Lines   []string // limit the run to these Rancher lines
	Preview bool     // print the commit message and PR body too
	NoPush  bool
	PushTo  string
	Yes     bool
	// Interactive says In is a terminal the question can be asked on.
	Interactive bool
	In          io.Reader
}

// RunSeries fixes the series holdings of a project: it checks each line, makes the
// commits for the ones an Edit follows and, unless --no-push or --dry-run, pushes them
// and opens the PRs after asking once. The report is written to w, as text or as one
// document in json, and it is written even when the error returned is ErrFailed or
// ErrDeclined.
func RunSeries(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts SeriesOptions) error {
	if err := checkFormat(s.Output); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "gero-ship-")
	if err != nil {
		return fmt.Errorf("make a directory for worktrees: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	ex := &domain.Executor{Git: domain.GitCLI{}, Runner: steprun.Runner{}, TempDir: tmp, Sign: s.SignCommits, Identify: identify(s)}
	if !opts.NoPush {
		creds, err := github.Resolve(github.GoGh{}, "")
		switch {
		case err == nil:
			ctx = pushContext(ctx, s, creds)
			remote, err := newRemote(creds, s)
			if err != nil {
				return err
			}
			ex.Publish = &domain.Publisher{Git: domain.GitCLI{}, Remote: remote}
			ex.Confirm = confirmer(opts.In, w, opts.Interactive, opts.Yes, style.New(s.Terminal))
		case s.DryRun && errors.Is(err, github.ErrNoToken):
			if log != nil {
				log.Warn("no GitHub token, so the open PRs were not looked at", "error", err)
			}
		default:
			return fmt.Errorf("opening PRs needs a GitHub token (--no-push does not): %w", err)
		}
	}
	in := domain.SeriesInput{
		Project: opts.Project, Holding: opts.Holding, Lines: opts.Lines, BranchPrefix: s.BranchPrefix,
		DryRun: s.DryRun, NoPush: opts.NoPush, PushTo: opts.PushTo, CI: s.CI,
	}
	return runSeriesTo(ctx, w, shipWorkspace{workspace.Open(s, cwd)}, ex, s.Output, in, opts.Preview, style.New(s.Terminal))
}

// runSeriesTo checks and fixes, with an executor, and writes the report. Errors that
// come after the check (no clone, a failed target, declined) are returned after it.
func runSeriesTo(ctx context.Context, w io.Writer, ws domain.Workspace, ex *domain.Executor, format string, in domain.SeriesInput, full bool, st *style.Renderer) error {
	if err := checkFormat(format); err != nil {
		return err
	}
	plan, err := domain.FixSeries(ctx, ws, in, ex)
	if errors.Is(err, workspace.ErrNotProject) {
		return fmt.Errorf("%w: name the project whose series holdings to fix as the first argument", err)
	}
	if err != nil && plan.Holder == "" {
		return err
	}
	out := renderSeriesText(plan, full, st)
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
	return seriesFailed(plan)
}

// seriesFailed is ErrFailed when a target failed, whether it could not be read or its
// commit or push did: the exit code says the run did not do everything it was asked.
func seriesFailed(p domain.SeriesPlan) error {
	for _, t := range p.Targets {
		if t.Status == domain.StatusFailed || (t.Result != nil && (t.Result.Status == domain.ResultFailed || t.Result.Status == domain.ResultBlocked)) {
			return ErrFailed
		}
	}
	return nil
}

// renderSeriesText is the report of a fix: a row for each line, and for each one that
// was worked on what was done, with the texts of the PR when full.
func renderSeriesText(p domain.SeriesPlan, full bool, view *style.Renderer) string {
	var b strings.Builder
	b.WriteString(view.Style(style.Heading, "Series: "+p.Holder) + "\n")
	if len(p.Targets) == 0 {
		b.WriteString("  " + view.Style(style.Dim, "no series holding") + "\n")
		return b.String()
	}
	table := [][]string{{"  " + view.Style(style.Dim, "LINE"), view.Style(style.Dim, "BRANCH"), view.Style(style.Dim, "HOLDING"),
		view.Style(style.Dim, "HOLDS"), view.Style(style.Dim, "WANTS"), view.Style(style.Dim, "STATUS")}}
	for _, t := range p.Targets {
		table = append(table, []string{"  " + t.Line, orDash(t.Branch), t.Holding, orDash(t.Current), orDash(t.Want), seriesStatus(view, t)})
	}
	b.WriteString(ui.Align(table, view.Width))
	for _, t := range p.Targets {
		if t.Result == nil && !(full && t.Preview != nil) {
			continue
		}
		b.WriteString("\n" + view.Style(style.Heading, t.Line+" on "+t.Branch) + "\n")
		if t.Preview != nil {
			b.WriteString("  branch  " + t.Preview.Branch + "\n  title   " + t.Preview.Title + "\n")
			if full {
				b.WriteString("\n" + indent(t.Preview.CommitMessage) + "\n" + indent(t.Preview.Body) + "\n")
			}
		}
		if r := t.Result; r != nil {
			b.WriteString("  " + view.Style(statusRoleOf(r.Status), string(r.Status)))
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
			for _, warning := range r.Warnings {
				b.WriteString("  " + view.Style(style.Warn, warning) + "\n")
			}
		}
	}
	return b.String()
}

// seriesStatus is how one line stands, in a few words.
func seriesStatus(view *style.Renderer, t domain.SeriesTarget) string {
	switch t.Status {
	case domain.StatusUpToDate:
		return view.Style(style.OK, "current")
	case domain.StatusReady:
		return view.Style(style.Warn, "differs, ready")
	case domain.StatusBlocked:
		return view.Style(style.Warn, "differs, left alone: "+t.Reason)
	}
	return view.Style(style.Error, "unreadable: "+t.Reason)
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
