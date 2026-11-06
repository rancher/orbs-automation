// Package repotag holds the logic of gero repo tag: making a tag on the head of
// the upstream branch of a release line, with the createtag task. It is
// framework-free: cmd/ owns the cobra wiring (docs/specs/TASK_COMMANDS.md).
package repotag

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/rancher/orbs-automation/internal/github/tags"
	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/rancher/orbs-automation/internal/task/createtag"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrNeedsApproval is returned when the tag is going to be made and there is no
// terminal to ask on and no --yes.
var ErrNeedsApproval = errors.New("this makes a tag and needs approval, but there is no terminal to ask on: pass --yes")

// ErrDeclined is returned when the answer to the question was no.
var ErrDeclined = errors.New("declined: nothing was tagged")

// ErrNoTag is returned when no tag was named. Choosing one is not built yet.
var ErrNoTag = errors.New("name the tag to make: choosing one is not built yet")

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Project string // by config name or slug, "" for the working directory's
	Line    string // the release line, or a branch of the project
	Tag     string // the tag to make
	Yes     bool   // go on without asking
	// Interactive says In is a terminal that the question can be asked on.
	Interactive bool
	In          io.Reader
}

// Result is what was done, as --json prints it.
type Result struct {
	Project string `json:"project"` // the repository, owner/name
	Branch  string `json:"branch"`  // the branch that was tagged
	Commit  string `json:"commit"`  // the head of it that was read
	Tag     string `json:"tag"`
	// Newest is the tag of the line that this one follows, when there is one.
	Newest string `json:"newest,omitempty"`
	// Action is "tagged", "already tagged" (it was on the commit), or "would tag"
	// for a dry run.
	Action string `json:"action"`
	DryRun bool   `json:"dryRun,omitempty"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed.
type Workspace interface {
	Project(key string) (config.Project, error)
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
}

// ParseArgs reads the positional arguments of gero repo tag: [project] <line>
// [tag]. A tag is always a full version, and a project or a line never is, which
// is how two arguments are told apart: a version second is the line and the tag,
// else they are the project and the line.
func ParseArgs(args []string) (project, line, tag string) {
	switch len(args) {
	case 1:
		return "", args[0], ""
	case 2:
		if _, isVersion := semver.Parse(args[1]); isVersion {
			return "", args[0], args[1]
		}
		return args[0], args[1], ""
	}
	return args[0], args[1], args[2]
}

// Run makes the tag opts names, and writes the report to w in the format the
// settings ask for. The token is only asked for when a tag is to be made.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	newRemote := func() (createtag.Remote, error) {
		creds, err := github.Resolve(github.GoGh{}, "")
		if err != nil {
			return nil, fmt.Errorf("making a tag needs a GitHub token: %w", err)
		}
		return tags.New(creds)
	}
	return run(ctx, w, workspace.Open(s, cwd), newRemote, s.Output, s.DryRun, opts)
}

// run is Run over its dependencies. The head of the line's branch and the tags
// are read from the host, so a tag is always on what is live there and a clone
// is not involved. What would be done is settled by the task's check before
// anything is asked, so a tag the line cannot have is an error and not a
// question, and a dry run needs no token.
func run(ctx context.Context, w io.Writer, ws Workspace, newRemote func() (createtag.Remote, error), format string, dryRun bool, opts Options) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by repo tag: use text or json", format)
	}
	if opts.Tag == "" {
		return ErrNoTag
	}
	project, err := ws.Project(opts.Project)
	if err != nil {
		return err
	}
	slug := project.Repo.Name
	branch, series, err := lineBranch(project, opts.Line)
	if err != nil {
		return err
	}
	refs, err := ws.UpstreamRefs(ctx, slug)
	if err != nil {
		return fmt.Errorf("read the branches and tags of %s: %w", slug, err)
	}
	head, ok := refs.Branch(branch)
	if !ok {
		return fmt.Errorf("%s has no branch %s on the host", slug, branch)
	}
	in := createtag.Input{Repo: slug, Tag: opts.Tag, Commit: head.Commit, Tags: refs.Tags, Pattern: project.Release.TagPattern, Series: series}
	decision, err := createtag.Check(in)
	if err != nil {
		return err
	}
	res := Result{Project: slug, Branch: branch, Commit: head.Commit, Tag: opts.Tag, DryRun: dryRun}
	if decision.Newest != nil {
		res.Newest = decision.Newest.Name
	}
	switch {
	case decision.Action == createtag.Already:
		res.Action = "already tagged"
	case dryRun:
		res.Action = "would tag"
	default:
		if err := approve(w, format, res, opts); err != nil {
			return err
		}
		remote, err := newRemote()
		if err != nil {
			return err
		}
		if _, err := createtag.Run(ctx, remote, in); err != nil {
			return err
		}
		res.Action = "tagged"
	}
	return report(w, format, res)
}

// approve asks once, before the tag is made, unless --yes was given. Without a
// terminal it is an error, so a forgotten flag in CI is a loud failure and not a
// hang or a silent yes, and the question is only ever asked in text, since json
// is for a program to read.
func approve(w io.Writer, format string, res Result, opts Options) error {
	if opts.Yes {
		return nil
	}
	if !opts.Interactive || format != clicfg.OutputText {
		return ErrNeedsApproval
	}
	if _, err := fmt.Fprintf(w, "Tag %s on %s at %s in %s? [y/N] ", res.Tag, res.Branch, short(res.Commit), res.Project); err != nil {
		return err
	}
	line, err := bufio.NewReader(opts.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return ErrDeclined
}

// report writes what was done.
func report(w io.Writer, format string, res Result) error {
	if format == clicfg.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	where := fmt.Sprintf("%s on %s at %s in %s", res.Tag, res.Branch, short(res.Commit), res.Project)
	var err error
	switch res.Action {
	case "tagged":
		_, err = fmt.Fprintf(w, "\nTagged %s\n  https://github.com/%s/releases/tag/%s\n", where, res.Project, res.Tag)
	case "would tag":
		_, err = fmt.Fprintf(w, "Would tag %s   (nothing was sent)\n", where)
	default:
		_, err = fmt.Fprintf(w, "%s is already tagged on %s at %s in %s: nothing to do\n", res.Tag, res.Branch, short(res.Commit), res.Project)
	}
	return err
}

// lineBranch finds the branch a release line lives on, and the series of
// versions the line has. The line may be written without the v, or be a branch of
// the project, which has the series of the line it is the branch of, or none when
// it is in no line.
func lineBranch(p config.Project, arg string) (branch, series string, err error) {
	if b, ok := p.BranchOfLine(arg); ok {
		for _, l := range p.Release.Lines {
			if strings.EqualFold(l.Line, withV(arg)) {
				return b, l.Series(), nil
			}
		}
	}
	if slices.Contains(p.ReleaseBranches(), arg) {
		for _, l := range p.Release.Lines {
			if l.Line == p.LineOfBranch(arg) {
				return arg, l.Series(), nil
			}
		}
		return arg, "", nil
	}
	var have []string
	for _, l := range p.Release.Lines {
		have = append(have, l.Line)
	}
	if len(have) == 0 {
		have = p.ReleaseBranches()
	}
	return "", "", fmt.Errorf("%s has no release line or branch %q: it has %s", p.Repo.Name, arg, strings.Join(have, ", "))
}

func withV(line string) string {
	if !strings.HasPrefix(strings.ToLower(line), "v") {
		return "v" + line
	}
	return line
}

// short abbreviates a commit for a message.
func short(commit string) string { return commit[:min(len(commit), 7)] }
