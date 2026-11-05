// Package ship holds the logic of gero ship: shipping a release of a producer
// to one consumer repository. For now it runs the state check and writes
// nothing; the write steps follow (docs/specs/PUSH_TO_REPO.md). It is
// framework-free: cmd/ owns the cobra wiring.
package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrBlocked is returned, with the report still written, when the state check
// finds something in the way, such as an image that is not published.
var ErrBlocked = errors.New("the release cannot be shipped yet")

// Options is what the user asked to ship; the command fills it from its
// arguments and flags.
type Options struct {
	Project    string // the producer, by config name or slug
	Arg        string // a tag, a release line of the producer, or empty for the latest
	To         string // the consumer, when several take updates from the producer
	Branch     string // the consumer's branch to update
	Prerelease bool   // let prereleases count
	Preview    bool   // print the commit message and PR body too, not only the branch and title
}

// Run checks the state of shipping what opts names, and renders the plan as
// s.Output (text or json). The plan is returned even with ErrBlocked.
func Run(ctx context.Context, s clicfg.Settings, cwd string, opts Options) (string, error) {
	if err := checkFormat(s.Output); err != nil {
		return "", err
	}
	return run(ctx, workspace.Open(s, cwd), registry.Checker{}, s.Output, domain.Input{
		Project: opts.Project, Arg: opts.Arg, To: opts.To, Branch: opts.Branch, Prerelease: opts.Prerelease,
	}, opts.Preview)
}

func checkFormat(format string) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by ship: use text or json", format)
	}
	return nil
}

func run(ctx context.Context, ws domain.Workspace, prober registry.Prober, format string, in domain.Input, full bool) (string, error) {
	if err := checkFormat(format); err != nil {
		return "", err
	}
	plan, err := domain.Check(ctx, ws, prober, in)
	if err != nil {
		return "", err
	}
	var out string
	if format == clicfg.OutputJSON {
		if out, err = renderJSON(plan); err != nil {
			return "", err
		}
	} else {
		out = renderText(plan, full)
	}
	if plan.Status == domain.StatusBlocked {
		return out, ErrBlocked
	}
	return out, nil
}

func renderText(p domain.Plan, full bool) string {
	var b strings.Builder
	kind := "stable"
	if !p.Release.Stable {
		kind = "prerelease"
	}
	fmt.Fprintf(&b, "Ship %s %s (%s) to %s %s\n\n", p.Producer, p.Release.Tag, kind, p.Consumer, p.Target.Branch)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	row := func(label, value string) { fmt.Fprintf(w, "  %s\t%s\n", label, value) }
	row("strategy", fmt.Sprintf("%s (%s)", p.Strategy, p.File))
	row("target", fmt.Sprintf("%s (%s)", p.Target.Branch, git.Short(p.Target.Commit)))
	row("now", p.Current)
	row("want", p.Want)
	row("status", string(p.Status))
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	for _, blocker := range p.Blockers {
		b.WriteString("  blocked: " + blocker + "\n")
	}
	if p.Images != nil && len(p.Images.Rows) > 0 {
		b.WriteString("\nImages\n")
		w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		for _, r := range p.Images.Rows {
			status := r.Status
			if r.Detail != "" {
				status += " (" + r.Detail + ")"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", r.Image, r.Label, status, r.Reference)
		}
		_ = w.Flush()
	}
	for _, warning := range p.Warnings {
		b.WriteString("\nWarning: " + warning + "\n")
	}
	b.WriteString("\nNothing was written.\n")
	return b.String()
}

func renderJSON(p domain.Plan) (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
