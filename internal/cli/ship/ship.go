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
	"slices"
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
	// AllowDowngrade ships a release older than what the consumer holds.
	AllowDowngrade bool
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
		BranchPrefix: s.BranchPrefix, AllowDowngrade: opts.AllowDowngrade, DryRun: s.DryRun,
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
	if errors.Is(err, workspace.ErrNotProject) {
		return "", fmt.Errorf("%w: name the project to ship as the first argument", err)
	}
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
	fmt.Fprintf(&b, "Ship %s %s (%s) to %s\n\n", p.Producer, p.Release.Tag, kind, p.Consumer)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  strategy\t%s (%s)\n  want\t%s\n  status\t%s\n", p.Strategy, p.File, p.Want, p.Status)
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	for _, blocker := range p.Blockers {
		b.WriteString("  blocked: " + blocker + "\n")
	}

	b.WriteString("\nTargets\n")
	w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  BRANCH\tHEAD\tNOW\tSTATUS")
	for _, t := range p.Targets {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", t.Branch, git.Short(t.Commit), t.Current, t.Status)
	}
	_ = w.Flush()
	for _, g := range groupTargets(p.Targets, func(t domain.TargetPlan) []string { return t.Blockers }) {
		fmt.Fprintf(&b, "  blocked (%s): %s\n", strings.Join(g.branches, ", "), g.text)
	}
	for _, t := range p.Targets {
		if t.Error != "" {
			fmt.Fprintf(&b, "  %s failed: %s\n", t.Branch, t.Error)
		}
	}

	if prs := previews(p); len(prs) > 0 {
		b.WriteString("\nPR\n")
		w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		for _, t := range prs {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", t.Branch, t.Preview.Branch, t.Preview.Title)
		}
		_ = w.Flush()
		if labels := prs[0].Preview.Labels; len(labels) > 0 {
			b.WriteString("  labels: " + strings.Join(labels, ", ") + "\n")
		}
		if full {
			for _, t := range prs {
				writeBlock(&b, "Commit message ("+t.Branch+")", t.Preview.CommitMessage)
				writeBlock(&b, "Body ("+t.Branch+")", t.Preview.Body)
			}
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
	for _, g := range groupTargets(p.Targets, func(t domain.TargetPlan) []string { return t.Warnings }) {
		fmt.Fprintf(&b, "\nWarning (%s): %s\n", strings.Join(g.branches, ", "), g.text)
	}
	if p.DryRun {
		b.WriteString("\nDry run: nothing was written, and no branch or PR was created.\n")
	} else {
		// Creating the branches and PRs is not built yet, so nothing is written
		// without --dry-run either.
		b.WriteString("\nNothing was written: creating the branches and PRs is not built yet.\n")
	}
	return b.String()
}

// group is one text and the branches it applies to.
type group struct {
	text     string
	branches []string
}

// groupTargets collects the texts of the targets (their blockers or warnings)
// and the branches each applies to, in order of first appearance, so the same
// reason for five branches is one line and not five.
func groupTargets(targets []domain.TargetPlan, texts func(domain.TargetPlan) []string) []group {
	var groups []group
	for _, t := range targets {
		for _, text := range texts(t) {
			i := slices.IndexFunc(groups, func(g group) bool { return g.text == text })
			if i < 0 {
				groups = append(groups, group{text: text})
				i = len(groups) - 1
			}
			groups[i].branches = append(groups[i].branches, t.Branch)
		}
	}
	return groups
}

// previews returns the targets that have a PR to show.
func previews(p domain.Plan) []domain.TargetPlan {
	var out []domain.TargetPlan
	for _, t := range p.Targets {
		if t.Preview != nil {
			out = append(out, t)
		}
	}
	return out
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
