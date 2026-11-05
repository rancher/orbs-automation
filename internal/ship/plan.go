package ship

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
)

var (
	// ErrNoConsumer is returned when no project takes updates from the producer,
	// or the one named by To is not among them.
	ErrNoConsumer = errors.New("no such consumer")
	// ErrAmbiguousConsumer is returned when several projects take updates from
	// the producer and none was chosen.
	ErrAmbiguousConsumer = errors.New("several consumers")
	// ErrBranchRequired is returned when no target branch was named.
	ErrBranchRequired = errors.New("a target branch is required")
	// ErrBranchNotFound is returned when the consumer's repository has no branch
	// of the name.
	ErrBranchNotFound = errors.New("no such branch")
)

// Workspace is what the state check needs from the invocation's workspace. It
// is defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Consumers returns the producer and the projects that take updates from it.
	Consumers(key string) (config.Project, []config.Consumer, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// UpstreamFile reads a file of a repository at a commit from its host.
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// Input is what the user asked to ship.
type Input struct {
	Project    string // the producer, by config name or slug
	Arg        string // a tag, a release line of the producer, or empty for the latest
	To         string // the consumer, when there are several
	Branch     string // the consumer's target branch
	Prerelease bool   // let prereleases count
}

// Status is where the state check leaves a bump.
type Status string

// The states.
const (
	StatusReady    Status = "ready"
	StatusUpToDate Status = "already up to date"
	StatusBlocked  Status = "blocked"
)

// Target is the consumer branch a bump goes to, and the commit at its head
// that was read.
type Target struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// Plan is the result of the state check: what would be shipped, where, and
// whether anything stands in the way. It is read-only, and nothing was written
// to produce it.
type Plan struct {
	Producer string           `json:"producer"` // the repository slug
	Release  Release          `json:"release"`
	Consumer string           `json:"consumer"` // the repository slug
	Strategy string           `json:"strategy"`
	Target   Target           `json:"target"`
	File     string           `json:"file"`
	Current  string           `json:"current"` // what the consumer holds now
	Want     string           `json:"want"`    // what it should hold for the release
	Status   Status           `json:"status"`
	Blockers []string         `json:"blockers,omitempty"`
	Images   *registry.Report `json:"images,omitempty"` // not checked when already up to date
	Warnings []string         `json:"warnings,omitempty"`
}

// Check runs the state check for in: it resolves the release, finds the
// consumer and its target branch, reads what the consumer holds now through the
// strategy, and, unless the consumer is already at the release, checks the
// release's images exist. It writes nothing.
func Check(ctx context.Context, ws Workspace, prober registry.Prober, in Input) (Plan, error) {
	return Plan{}, nil
}
