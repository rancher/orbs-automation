// Package repoclone holds the logic of gero repo clone: putting the team's
// projects on a workstation, each under the clone root. It is framework-free:
// cmd/ owns the cobra wiring (docs/specs/CLI_FRAMEWORK.md).
package repoclone

import (
	"context"
	"io"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Projects []string // by config name or slug; none with All
	All      bool     // every Project that is owned and not external
	Into     string   // --into: the clone root, which beats the environment and the user file
	// Style colors the text report. Nil is plain, and so is a renderer built for a
	// terminal without color; the JSON report is never styled.
	Style *style.Renderer
}

// Outcome says what became of one Project.
const (
	Cloned  = "cloned"  // a new clone was made
	Skipped = "skipped" // there was a clone already
	Failed  = "failed"  // nothing was made, and Error says why
)

// Result is what became of one Project, as --json prints it.
type Result struct {
	Project string `json:"project"` // the repository, owner/name
	Outcome string `json:"outcome"` // Cloned, Skipped or Failed
	Path    string `json:"path,omitempty"`
	// Remotes are the remotes the clone has, in the order they were made: the
	// fork then the upstream, or the upstream alone.
	Remotes []string `json:"remotes,omitempty"`
	// Note says what the outcome does not, such as that the fork was not found and
	// the upstream alone was cloned.
	Note   string `json:"note,omitempty"`
	Error  string `json:"error,omitempty"`
	DryRun bool   `json:"dryRun,omitempty"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed.
type Workspace interface {
	UpstreamFor(slug string) string
	ConfiguredRepoPath(slug string) (workspace.RepoPath, error)
}

// ProjectSource is where the Projects to clone come from. Today that is the local
// central config; a source that reads the central bundle can stand in for it.
// TODO(bundle): add a bundle-backed source, for a workstation with no central
// config, and the newer of the two when both exist (docs/specs/CENTRAL_BUNDLE.md).
type ProjectSource interface {
	// Project is the Project named by its config name or slug.
	Project(key string) (*chain.Project, error)
	// Projects are all of them.
	Projects() ([]*chain.Project, error)
}

// Git is what the command needs of git. It is defined here, where it is consumed.
type Git interface {
	Clone(ctx context.Context, url, dir, remote string) error
	AddRemote(ctx context.Context, dir, name, url string) error
	RemoteURL(ctx context.Context, dir, name string) (string, error)
}

// Forks says whether the host has a fork.
type Forks interface {
	// ForkExists is whether user has a fork of the repository slug.
	ForkExists(ctx context.Context, user, slug string) (bool, error)
}

// Deps are what the command runs on, so a test can stand in for each.
type Deps struct {
	Workspace Workspace
	Source    ProjectSource
	Git       Git
	Forks     Forks
}

// Run clones what opts asks for and writes the report to w in the format the
// settings ask for. It returns an error after the report when any Project failed.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	return nil
}

func run(ctx context.Context, w io.Writer, d Deps, s clicfg.Settings, cwd string, opts Options) error {
	return nil
}
