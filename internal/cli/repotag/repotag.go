// Package repotag holds the logic of gero repo tag: making a tag on the head of
// the upstream branch of a release line, with the createtag task. It is
// framework-free: cmd/ owns the cobra wiring (docs/specs/TASK_COMMANDS.md).
package repotag

import (
	"context"
	"errors"
	"io"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/task/createtag"
)

// ErrNeedsApproval is returned when the tag is going to be made and there is no
// terminal to ask on and no --yes.
var ErrNeedsApproval = errors.New("this makes a tag and needs approval, but there is no terminal to ask on: pass --yes")

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
//
// STUB: reads nothing until a later commit.
func ParseArgs(args []string) (project, line, tag string) { return "", "", "" }

// Run makes the tag opts names, and writes the report to w in the format the
// settings ask for.
//
// STUB: does nothing until a later commit.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	return nil
}

// run is Run over its dependencies.
//
// STUB: does nothing until a later commit.
func run(ctx context.Context, w io.Writer, ws Workspace, newRemote func() (createtag.Remote, error), format string, dryRun bool, opts Options) error {
	return nil
}
