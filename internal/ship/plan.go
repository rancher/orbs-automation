package ship

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

var (
	// ErrNoConsumer is returned when no project takes updates from the producer,
	// or the one named by To is not among them.
	ErrNoConsumer = errors.New("no such consumer")
	// ErrAmbiguousConsumer is returned when the producer has several routes and
	// none was chosen.
	ErrAmbiguousConsumer = errors.New("several consumers")
	// ErrBranchRequired is returned when no target branch was named.
	ErrBranchRequired = errors.New("a target branch is required")
	// ErrNoTargets is returned when no consumer branch can be worked out for the
	// release, and none was named.
	ErrNoTargets = errors.New("no target branch for the release")
	// ErrBranchNotFound is returned when the consumer's repository has no branch
	// of the name.
	ErrBranchNotFound = errors.New("no such branch")
)

// Workspace is what the state check needs from the invocation's workspace. It
// is defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the project of a config name or slug, or the one the working
	// directory is in when the key is empty.
	Project(key string) (config.Project, error)
	// Updates builds the update dispatcher's engine for the central config. It
	// may return an engine along with an error about methods or edges that are
	// not usable.
	Updates() (*updatedispatcher.Engine, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// UpstreamFile reads a file of a repository at a commit from its host.
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
	// CloneDir is the directory of the engineer's clone of a repository, or an
	// error saying how to get one.
	CloneDir(slug string) (string, error)
	// Remote is the name of the remote of a repository's clone that is its
	// upstream.
	Remote(slug string) (string, error)
}

// Input is what the user asked to ship.
type Input struct {
	Project    string // the producer, by config name or slug
	Arg        string // a tag, a release line of the producer, or empty for the latest
	To         string // the consumer, when the producer ships to several
	Method     string // the consumer's update method, when the producer uses several of one consumer
	Branch     string // the consumer's target branch
	Prerelease bool   // let prereleases count
	// DryRun is the invocation's --dry-run: compute and report, and create no
	// branch or PR. It is carried into the plan so the report says which it is.
	DryRun bool
	// NoPush stops after the commit: the branch is left in the clone for review,
	// and nothing is pushed and no PR is created. With DryRun it is a dry run.
	NoPush bool
	// AllowDowngrade ships a release older than what the consumer holds, which is
	// otherwise a blocker.
	AllowDowngrade bool
	// BranchPrefix is the user's branchPrefix setting. When set it replaces the
	// project's whole, and it is empty when the user has none.
	BranchPrefix string
}

// Status is where the state check leaves a bump, and each target of it.
type Status string

// The states.
const (
	StatusReady    Status = "ready"
	StatusUpToDate Status = "already up to date"
	StatusBlocked  Status = "blocked"
	// StatusFailed is a target that could not be checked: its branch or file
	// could not be read. It is reported and the other targets go on.
	StatusFailed Status = "failed"
)

// Target is the consumer branch a bump goes to, and the commit at its head
// that was read.
type Target struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// TargetPlan is the state of one consumer branch: what it holds, what it should
// hold, and whether a PR is needed.
type TargetPlan struct {
	Target
	Current   string   `json:"current"` // what the consumer holds now
	File      string   `json:"-"`       // the file the method edits; the plan carries it once
	Want      string   `json:"-"`       // what it should hold; the same for every branch
	Status    Status   `json:"status"`
	Downgrade bool     `json:"downgrade,omitempty"` // the release is older than what the consumer holds
	Blockers  []string `json:"blockers,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Error     string   `json:"error,omitempty"`   // why a failed target could not be checked
	Preview   *Preview `json:"preview,omitempty"` // what a run would write; none when nothing would be
	// Result is what a run did for this target: set only by Ship, and only for a
	// target that was ready.
	Result *TargetResult `json:"result,omitempty"`
}

// Plan is the result of the state check: what would be shipped, to which
// branches, and whether anything stands in the way. It is read-only, and
// nothing was written to produce it.
type Plan struct {
	Producer string           `json:"producer"` // the repository slug
	Release  Release          `json:"release"`
	Consumer string           `json:"consumer"` // the repository slug
	Method   string           `json:"method"`   // the consumer's update method
	Strategy string           `json:"strategy"` // what the method uses to read and write the value
	File     string           `json:"file"`     // the consumer's file the strategy edits
	Want     string           `json:"want"`     // what the consumer should hold for the release
	DryRun   bool             `json:"dryRun"`   // the run was a dry run: no branch or PR is created
	NoPush   bool             `json:"noPush"`   // the run stops after the commit, with the branch left in the clone; never set with DryRun
	Status   Status           `json:"status"`   // blocked if any target is blocked or failed or the release has a blocker, else ready if any target needs a PR
	Targets  []TargetPlan     `json:"targets"`
	Blockers []string         `json:"blockers,omitempty"` // in the way of the whole release
	Images   *registry.Report `json:"images,omitempty"`   // checked once, and only when some target needs a PR
	Warnings []string         `json:"warnings,omitempty"`
	// Clone and Remote are the engineer's clone of the consumer and its upstream
	// remote, set by Ship when it made commits, so the report can say how to
	// review them.
	Clone  string `json:"clone,omitempty"`
	Remote string `json:"remote,omitempty"`
}

// Check runs the state check for in. It resolves the release, finds the
// consumer, and for each target branch reads what the consumer holds through the
// strategy; if any branch needs a PR it checks the release's images exist, once.
// It writes nothing.
func Check(ctx context.Context, ws Workspace, prober registry.Prober, in Input) (Plan, error) {
	plan, _, err := check(ctx, ws, prober, in, nil)
	return plan, err
}

// check is Check, and the resolution it was made from, which a run goes on to use.
func check(ctx context.Context, ws Workspace, prober registry.Prober, in Input, sink Sink) (Plan, resolution, error) {
	res, err := resolve(ctx, ws, in)
	if err != nil {
		return Plan{}, resolution{}, err
	}
	branches, named, err := res.branches(in)
	if err != nil {
		return Plan{}, resolution{}, err
	}
	plan := Plan{
		Producer: res.producer.Repo.Name, Release: res.release, Consumer: res.route.Consumer.Repo.Name,
		Method: res.route.Method.Name, Strategy: res.route.Method.StrategyName, DryRun: in.DryRun, NoPush: in.NoPush && !in.DryRun,
	}
	header := plan
	emit(sink, Event{Kind: EventResolved, Plan: &header})
	needsPR := false
	for _, branch := range branches {
		tp, err := res.checkTarget(ctx, ws, branch, in)
		if err != nil {
			if named {
				return Plan{}, resolution{}, err // asked for this branch, so not being able to read it is the answer
			}
			// One branch of several that could not be read does not stop the
			// others: it is reported as failed.
			tp = TargetPlan{Target: Target{Branch: branch}, Status: StatusFailed, Error: err.Error()}
		}
		needsPR = needsPR || tp.Status == StatusReady
		if plan.Want == "" {
			plan.File, plan.Want = tp.File, tp.Want // the same for every branch
		}
		plan.Targets = append(plan.Targets, tp)
		checked := tp
		emit(sink, Event{Kind: EventTargetChecked, Branch: tp.Branch, Target: &checked})
	}
	emit(sink, Event{Kind: EventTargetsChecked})
	if needsPR {
		images := registry.CheckProject(ctx, res.producer, prober, res.release.Tag)
		plan.Images, plan.Warnings = &images, images.Warnings
		if images.Failed() {
			plan.Blockers = append(plan.Blockers, "images: the release is not fully published")
		}
		emit(sink, Event{Kind: EventImagesChecked, Images: &images})
	}
	plan.Status = overall(plan)
	return plan, res, nil
}

// overall is the state of the whole plan.
func overall(p Plan) Status {
	if len(p.Blockers) > 0 {
		return StatusBlocked
	}
	status := StatusUpToDate
	for _, t := range p.Targets {
		switch t.Status {
		case StatusBlocked, StatusFailed:
			return StatusBlocked
		case StatusReady:
			status = StatusReady
		}
	}
	return status
}
