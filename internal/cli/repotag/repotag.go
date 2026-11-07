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
	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/task/createtag"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrNeedsApproval is returned when the tag is going to be made and there is no
// terminal to ask on and no --yes.
var ErrNeedsApproval = errors.New("this makes a tag and needs approval, but there is no terminal to ask on: pass --yes")

// Bump is how much a new version rises after a stable tag, as the command line
// says it. The zero value is a patch.
type Bump int

// The bumps.
const (
	BumpPatch Bump = iota
	BumpMinor
	BumpMajor
)

func (b Bump) semver() semver.Bump {
	switch b {
	case BumpMinor:
		return semver.Minor
	case BumpMajor:
		return semver.Major
	}
	return semver.Patch
}

// ErrDeclined is returned when the answer to the question was no.
var ErrDeclined = errors.New("declined: nothing was tagged")

// ErrTagAndChoice is returned when a tag is named and so is how to choose one.
var ErrTagAndChoice = errors.New("name the tag, or ask for one to be chosen with --stable, --minor or --major, and not both")

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Project string // by config name or slug, "" for the working directory's
	Line    string // the release line, or a branch of the project
	Tag     string // the tag to make; "" to have one chosen
	// Stable and Bump say what to choose when no tag is named: a stable tag and not
	// a prerelease, and how much a new version rises after a stable tag. They are
	// not used with a tag.
	Stable bool
	Bump   Bump
	// Push sends the tag to the upstream as well, after asking once.
	Push bool
	Yes  bool // go on without asking before a push
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
	// Promotes is the prerelease that a stable tag is the promotion of.
	Promotes string `json:"promotes,omitempty"`
	// Action is "tagged" (in the clone only), "tagged and pushed", "pushed" (the tag
	// was in the clone already), "already tagged" (the host has it), or for a dry
	// run "would tag", "would tag and push" or "would push".
	Action string `json:"action"`
	// Remote is the upstream remote the tag was or would be pushed to.
	Remote string `json:"remote,omitempty"`
	DryRun bool   `json:"dryRun,omitempty"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed.
type Workspace interface {
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	UpstreamFor(slug string) string
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
}

// Git is what the command needs from git, on top of making the tag. It is
// defined here, where it is consumed.
type Git interface {
	createtag.Git
	// Fetch fetches one branch of the remote into the clone and returns its commit.
	Fetch(ctx context.Context, dir, remote, branch string) (string, error)
	// TagCommit is the commit a local tag is on, and false when there is none.
	TagCommit(ctx context.Context, dir, name string) (commit string, found bool, err error)
	// PushTag sends a tag to the remote, never forced.
	PushTag(ctx context.Context, dir, remote, name string) error
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

// Run makes the tag opts names in the engineer's clone, and with --push sends it
// to the upstream, and writes the report to w in the format the settings ask for.
// Nothing here needs a GitHub token: the head and the tags are read from the host
// without one, and a push is git's, with the engineer's own credentials.
//
// STUB: does nothing until a later commit.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	return run(ctx, w, workspace.Open(s, cwd), ship.GitCLI{}, s.Output, s.DryRun, opts)
}

// run is Run over its dependencies.
//
// STUB: does nothing until a later commit.
func run(ctx context.Context, w io.Writer, ws Workspace, g Git, format string, dryRun bool, opts Options) error {
	return nil
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
	if _, err := fmt.Fprintf(w, "Tag %s? [y/N] ", res.what()); err != nil {
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
	where := res.what()
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

// what says which tag is made on what, as the question and the report put it. A
// promotion says which prerelease it is the stable version of, and that the head
// has not moved since, which is the reason it is allowed.
func (r Result) what() string {
	if r.Promotes != "" {
		return fmt.Sprintf("%s on %s at %s, which is %s with no commits since, in %s", r.Tag, r.Branch, short(r.Commit), r.Promotes, r.Project)
	}
	return fmt.Sprintf("%s on %s at %s in %s", r.Tag, r.Branch, short(r.Commit), r.Project)
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
