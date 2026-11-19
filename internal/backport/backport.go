package backport

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
)

// Status is where a target stands.
type Status string

// The states of a target. What a run did for one that was worked on is its Result.
const (
	// StatusReady: the picks are known and a PR is to be made.
	StatusReady Status = "ready"
	// StatusAlreadyBackported: the history of the target already holds the picked commits.
	StatusAlreadyBackported Status = "already backported"
	// StatusAborted: a conflict stopped the pick and the person abandoned it.
	StatusAborted Status = "aborted"
	// StatusNotAttempted: an earlier target was aborted, and nothing after it was tried.
	StatusNotAttempted Status = "not attempted"
)

// ErrAborted is the failure of a target whose conflict was abandoned.
var ErrAborted = errors.New("the cherry-pick was abandoned")

// Host is what a backport reads of the PR on its host.
type Host interface {
	Get(ctx context.Context, slug string, number int) (pulls.Detail, error)
	Commits(ctx context.Context, slug string, number int) ([]pulls.Commit, error)
}

// Workspace is what a backport needs: the projects, what the host has, the PR, and the
// engineer's clone of the project and where to push.
type Workspace interface {
	// Project returns the project of a config name or slug, or the one the working
	// directory is in when the key is empty.
	Project(key string) (*chain.Project, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// PullRequests reads PRs from the host.
	PullRequests() Host
	proposal.Site
}

// Choice is what a person decides about a conflict.
type Choice int

// The two answers.
const (
	// Continue: the conflicts are resolved in the work directory.
	Continue Choice = iota + 1
	// Abort: abandon the pick, and with it the run.
	Abort
)

// Conflict is a pick that stopped, for a person to resolve.
type Conflict struct {
	Branch  string   // the target branch
	Line    string   // its Rancher line, when it has one
	Commit  string   // the commit that was being picked
	Subject string   // its first line
	Dir     string   // the work directory, where the files are
	Files   []string // the files with conflicts
	// Left are the files that still hold a conflict marker after an earlier Continue.
	Left []string
}

// Resolver waits for a person to resolve a conflict, or abandons it when there is nobody to
// ask. It returns when the person says which.
type Resolver interface {
	Resolve(ctx context.Context, c Conflict) (Choice, error)
}

// Input is what the user asked to backport.
type Input struct {
	Project string   // the repo, by config name or slug; empty for the one the working directory is in
	Lines   []string // the targets: Rancher lines or release branches, as typed
	PR      int      // the number of the merged PR
	// BranchPrefix is the user's branchPrefix setting, which replaces the project's.
	BranchPrefix string
	// DefaultBranchPrefix is used when neither BranchPrefix nor the project says one.
	DefaultBranchPrefix string
	DryRun              bool   // compute and report, and create no branch or PR
	NoPush              bool   // stop after the commits, leaving the branches in the clone
	PushTo              string // fork or upstream, or empty to let the settings decide
	CI                  bool   // the actor is the bot
	// Resolver is asked about each conflict. Nil abandons them.
	Resolver Resolver
	// Git is the git of the picks. Nil is GitCLI.
	Git Git
}

// Plan is what a backport would do, and what a run did.
type Plan struct {
	Repo    string               `json:"repo"` // the repository, owner/name
	PR      int                  `json:"pr"`
	Title   string               `json:"title"`
	URL     string               `json:"url"`
	Base    string               `json:"base"`
	Shape   Shape                `json:"shape,omitempty"`
	Picks   []string             `json:"picks,omitempty"`
	Clone   string               `json:"clone,omitempty"`
	Remote  string               `json:"remote,omitempty"`
	Push    *proposal.PushTarget `json:"push,omitempty"`
	Actor   *proposal.Actor      `json:"actor,omitempty"`
	Targets []Target             `json:"targets,omitempty"`
}

// Target is one release branch the PR is carried to.
type Target struct {
	Branch string `json:"branch"`
	Line   string `json:"line,omitempty"`
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	Status Status `json:"status"`
	// Files are the files that had conflicts, resolved by hand.
	Files   []string          `json:"conflicts,omitempty"`
	Preview *proposal.Preview `json:"preview,omitempty"`
	// Result is what a run did for a target that was worked on.
	Result *proposal.TargetResult `json:"result,omitempty"`
}

// Run carries a merged PR to the targets, one PR for each, newest line first. Each target is
// checked against the host, its picks are made in the clone or a worktree of it, and, unless
// --no-push or --dry-run, it is pushed and opened as a PR after asking once. A conflict stops
// the whole run until the person has resolved it or abandoned it; abandoning ends the run,
// and the targets not yet tried are not attempted.
func Run(ctx context.Context, ws Workspace, in Input, ex *proposal.Executor) (Plan, error) {
	return Plan{}, nil
}
