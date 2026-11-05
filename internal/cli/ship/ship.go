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
	"regexp"
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

// ParseArgs reads the positional arguments of gero ship: [project] [tag|line].
// A single argument is a project unless it looks like a release, a version
// such as v0.5.3 or a line such as v2.15 (no project is named like that), in
// which case the project is the one the working directory is in and the
// argument is the release. No arguments is that project's latest release.
func ParseArgs(args []string) (project, arg string) {
	switch len(args) {
	case 0:
		return "", ""
	case 1:
		if looksLikeARelease(args[0]) {
			return "", args[0]
		}
		return args[0], ""
	}
	return args[0], args[1]
}

// release matches a version or a line: v0.5.3, 0.6.0-rc.1, v2.15.
var release = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){1,2}([-+].*)?$`)

func looksLikeARelease(s string) bool { return release.MatchString(s) }

// Run checks the state of shipping what opts names, and renders the plan as
// s.Output (text or json). The plan is returned even with ErrBlocked.
func Run(ctx context.Context, s clicfg.Settings, cwd string, opts Options) (string, error) {
	if err := checkFormat(s.Output); err != nil {
		return "", err
	}
	return run(ctx, workspace.Open(s, cwd), registry.Checker{}, s.Output, domain.Input{
		Project: opts.Project, Arg: opts.Arg, To: opts.To, Branch: opts.Branch, Prerelease: opts.Prerelease,
		BranchPrefix: s.BranchPrefix,
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
	if p.Preview != nil {
		b.WriteString("\nPR\n")
		w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  branch\t%s\n  title\t%s\n", p.Preview.Branch, p.Preview.Title)
		if len(p.Preview.Labels) > 0 {
			fmt.Fprintf(w, "  labels\t%s\n", strings.Join(p.Preview.Labels, ", "))
		}
		_ = w.Flush()
		if full {
			writeBlock(&b, "Commit message", p.Preview.CommitMessage)
			writeBlock(&b, "Body", p.Preview.Body)
		}
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

// writeBlock writes a heading and the text under it, indented.
func writeBlock(b *strings.Builder, heading, text string) {
	b.WriteString("\n" + heading + "\n")
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("  " + line + "\n")
	}
}

func renderJSON(p domain.Plan) (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
