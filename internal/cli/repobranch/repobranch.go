// Package repobranch holds the logic of gero repo branch: making a branch off
// the upstream branch of a release line, with the createbranch task. It is
// framework-free: cmd/ owns the cobra wiring (docs/specs/TASK_COMMANDS.md).
package repobranch

import (
	"context"
	"io"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/task/createbranch"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Project string // by config name or slug, "" for the working directory's
	Line    string // the release line, or a branch of the project
	Name    string // the new branch
	Switch  bool   // leave the checkout on the new branch
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
func ParseArgs(_ []string) (project, line, name string) {
	return "", "", ""
}

// Run makes the branch opts names, in the engineer's clone, and writes the
// report to w in the format the settings ask for.
func Run(_ context.Context, _ io.Writer, _ clicfg.Settings, _ string, _ Options) error {
	return nil
}

func run(_ context.Context, _ io.Writer, _ Workspace, _ createbranch.Git, _, _ string, _ bool, _ Options) error {
	return nil
}
