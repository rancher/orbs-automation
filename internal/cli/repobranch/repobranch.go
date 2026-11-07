// Package repobranch holds the logic of gero repo branch: making a branch off
// the upstream branch of a release line, with the createbranch task. It is
// framework-free: cmd/ owns the cobra wiring (docs/specs/TASK_COMMANDS.md).
package repobranch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/task/createbranch"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Project string // by config name or slug, "" for the working directory's
	Line    string // the release line, or a branch of the project
	Name    string // the new branch
	Switch  bool   // leave the checkout on the new branch
	// Style colors the text report. Nil is plain, and so is a renderer built for a
	// terminal without color; the JSON report is never styled.
	Style *style.Renderer
}

// Result is what was done, as --json prints it.
type Result struct {
	Project  string `json:"project"` // the repository, owner/name
	Branch   string `json:"branch"`  // the new branch
	From     string `json:"from"`    // remote/branch it was cut from
	Base     string `json:"base"`    // the commit it was cut at
	Switched bool   `json:"switched,omitempty"`
	// Existing says the branch was already there, ours and on the current head, so
	// nothing was made.
	Existing bool `json:"existing,omitempty"`
	DryRun   bool `json:"dryRun,omitempty"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed.
type Workspace interface {
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	UpstreamFor(slug string) string
}

// ParseArgs reads the positional arguments of gero repo branch: [project]
// <line> <name>. With two they are the line and the name.
func ParseArgs(args []string) (project, line, name string) {
	if len(args) == 2 {
		return "", args[0], args[1]
	}
	return args[0], args[1], args[2]
}

// Run makes the branch opts names, in the engineer's clone, and writes the
// report to w in the format the settings ask for.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	tmp, err := os.MkdirTemp("", "gero-branch-")
	if err != nil {
		return fmt.Errorf("make a directory for a worktree: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	opts.Style = style.New(s.Terminal)
	return run(ctx, w, workspace.Open(s, cwd), ship.GitCLI{}, tmp, s.Output, s.DryRun, opts)
}

func run(ctx context.Context, w io.Writer, ws Workspace, g createbranch.Git, tmp, format string, dryRun bool, opts Options) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by repo branch: use text or json", format)
	}
	project, err := ws.Project(opts.Project)
	if err != nil {
		return err
	}
	slug := project.Repo.Name
	remote := ws.UpstreamFor(slug)
	if remote == "" {
		return fmt.Errorf("no upstream remote is set for %s: set upstream in the user file, or repos.overrides.%s.upstream", slug, slug)
	}
	target, err := lineBranch(project, opts.Line)
	if err != nil {
		return err
	}
	clone, err := ws.RepoPath(slug)
	if err != nil {
		return fmt.Errorf("%s: clone it under a repos.search directory, or set repos.paths.%s (%v)", slug, slug, err)
	}
	base, err := g.Fetch(ctx, clone.Path, remote, target)
	if err != nil {
		return err
	}
	out, err := createbranch.Run(ctx, g, createbranch.Input{
		CloneDir: clone.Path, Remote: remote, Target: target, Base: base, Branch: opts.Name, DryRun: dryRun, TempDir: tmp,
	})
	if errors.Is(err, createbranch.ErrBranchCheckedOut) {
		return fmt.Errorf("%w: choose another name", err)
	}
	if err != nil {
		return err
	}
	work := out.Work
	// The branch stays, unless this was only to say what would be done. The
	// checkout goes back to where it was, which is why the engineer's own work is
	// never in the way.
	if err := work.Finish(ctx, !dryRun); err != nil {
		return err
	}
	res := Result{Project: slug, Branch: opts.Name, From: remote + "/" + target, Base: base, Existing: work.Reused, DryRun: dryRun}
	if opts.Switch && !dryRun {
		if err := g.Checkout(ctx, clone.Path, opts.Name); err != nil {
			return fmt.Errorf("the branch %s is made, but switching to it failed: %w", opts.Name, err)
		}
		res.Switched = true
	}
	if format == clicfg.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	_, err = fmt.Fprint(w, render(res, len(work.Existing), opts.Style))
	return err
}

// lineBranch is the branch of the project that a release line lives on: the
// line's own branch, or the default branch for a line that has none. A branch of
// the project's, such as release/v2.15 or main, is taken as it is.
func lineBranch(p config.Project, line string) (string, error) {
	if branch, ok := p.BranchOfLine(line); ok {
		return branch, nil
	}
	branches := p.ReleaseBranches()
	if slices.Contains(branches, line) {
		return line, nil
	}
	var lines []string
	for _, l := range p.Release.Lines {
		lines = append(lines, l.Line)
	}
	if len(lines) == 0 {
		lines = branches
	}
	return "", fmt.Errorf("%s has no release line or branch %q: it has %s", p.Repo.Name, line, strings.Join(lines, ", "))
}

// render is the report in text. What was made is green, what only would be is
// secondary text, and so is the hint to switch to the branch.
func render(r Result, existing int, view *style.Renderer) string {
	short := r.Base[:min(len(r.Base), 8)]
	switch {
	case r.Existing:
		noun := "commits"
		if existing == 1 {
			noun = "commit"
		}
		return view.Style(style.Dim, fmt.Sprintf("%s already exists in %s with %d %s of ours on %s: nothing to do", r.Branch, r.Project, existing, noun, r.From)) + "\n"
	case r.DryRun:
		return fmt.Sprintf("%s %s at %s (%s) in %s\n", view.Style(style.Dim, "Would create"), r.Branch, r.From, short, r.Project)
	case r.Switched:
		return fmt.Sprintf("%s %s at %s (%s) in %s, and switched to it\n", view.Style(style.OK, "Created"), r.Branch, r.From, short, r.Project)
	}
	return fmt.Sprintf("%s %s at %s (%s) in %s\n  %s\n", view.Style(style.OK, "Created"), r.Branch, r.From, short, r.Project,
		view.Style(style.Dim, "git switch "+r.Branch))
}
