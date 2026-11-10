package ship

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/registry"
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
	// ErrNotOwned is returned when the producer is not owned by the active team: gero
	// reads where its releases stand and never opens a PR for them.
	ErrNotOwned = errors.New("not owned")
	// ErrBranchNotFound is returned when the consumer's repository has no branch
	// of the name.
	ErrBranchNotFound = errors.New("no such branch")
)

// Reader is what reading the state of a bump needs from the invocation's
// workspace: the projects and the update engine, and what the host has. It is
// all that gero repo status needs to trace the consumers. It is defined here,
// where it is consumed, so tests can use a fake.
type Reader interface {
	// Project returns the project of a config name or slug, or the one the
	// working directory is in when the key is empty.
	Project(key string) (*chain.Project, error)
	// Bump builds the update engine for the central config. It may return an
	// engine along with an error about edits or holdings that are not usable.
	Bump() (*bump.Engine, error)
	// UpstreamRefs lists the tags and branch heads of a repository on its host.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// UpstreamFile reads a file of a repository at a commit from its host.
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// Workspace is what a run needs from the invocation's workspace: what is read
// from the host, and the engineer's clone of the consumer and where to push.
type Workspace interface {
	Reader
	// CloneDir is the directory of the engineer's clone of a repository, or an
	// error saying how to get one.
	CloneDir(slug string) (string, error)
	// Remote is the name of the remote of a repository's clone that is its
	// upstream.
	Remote(slug string) (string, error)
	// PushSettings are the user's settings that decide where a branch for a
	// repository is pushed.
	PushSettings(slug string) PushSettings
}

// PushSettings are the user's settings that decide where a branch is pushed, as
// the workspace has them.
type PushSettings struct {
	UserRepo    string // repos.overrides.<slug>.pushTo
	UserDefault string // pushTo
	Fork        string // the remote that holds the engineer's fork
	Upstream    string // the remote that is the real repository
	GitHubUser  string // the owner of the fork, for a PR's head
}

// Input is what the user asked to ship.
type Input struct {
	Project    string   // the producer, by config name or slug
	Arg        string   // a tag, a release line of the producer, or empty for the latest
	To         string   // the consumer, when the producer ships to several
	Lines      []string // the consumer's Rancher lines to ship to, such as v2.15, in place of every branch the release goes to
	Prerelease bool     // let prereleases count
	// Team is the active team context: a producer it owns is shipped from.
	Team string
	// DryRun is the invocation's --dry-run: compute and report, and create no
	// branch or PR. It is carried into the plan so the report says which it is.
	DryRun bool
	// PushTo is --push-to: fork or upstream, or "" to let the settings decide.
	PushTo string
	// Refresh rebuilds an already open PR of the actor's whose branch differs from
	// what a fresh build would make, and replaces its branch.
	Refresh bool
	// CI says the actor is the bot, in CI: it always pushes to the upstream and
	// the bot's labels are added.
	CI bool
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
	// Line is the Rancher line the consumer's branch is the branch of, such as
	// v2.15, or "" for a branch that is in no line. It is how a report names a
	// target, since a line means the same in every project and a branch does not.
	Line    string `json:"line,omitempty"`
	Release string `json:"release"` // the tag of the release that goes to this branch
	// Holdings is what each holding written in this branch holds now and should
	// hold for the release, in the order the config reads them.
	Holdings  []HoldingPlan `json:"holdings,omitempty"`
	Status    Status        `json:"status"`
	Downgrade bool          `json:"downgrade,omitempty"` // the release is older than what the consumer holds
	Blockers  []string      `json:"blockers,omitempty"`
	Warnings  []string      `json:"warnings,omitempty"`
	Error     string        `json:"error,omitempty"`   // why a failed target could not be checked
	Preview   *Preview      `json:"preview,omitempty"` // what a run would write; none when nothing would be
	// Result is what a run did for this target: set only by Ship, and only for a
	// target that was ready.
	Result *TargetResult `json:"result,omitempty"`
}

// HoldingPlan is the state of one holding in a consumer branch.
type HoldingPlan struct {
	Name      string `json:"name"`    // the holding
	File      string `json:"file"`    // the consumer's file it is in
	Current   string `json:"current"` // what the file holds now
	Want      string `json:"want"`    // what it should hold for the release
	UpToDate  bool   `json:"upToDate"`
	Downgrade bool   `json:"downgrade,omitempty"` // the release is older than what the file holds
	// Pinned says the holding is a companion's: the virtual Holding a pin makes of
	// another module, written with the release the pin resolved.
	Pinned bool `json:"pinned,omitempty"`
}

// Key is what a report calls the target: its Rancher line, which means the same
// in every project, or its branch when it is in no line.
func (t TargetPlan) Key() string {
	if t.Line != "" {
		return t.Line
	}
	return t.Branch
}

// Plan is the result of the state check: what would be shipped, to which
// branches, and whether anything stands in the way. It is read-only, and
// nothing was written to produce it.
type Plan struct {
	Producer string `json:"producer"` // the repository slug
	// Releases are the releases that go to the targets, each once, in the order
	// of the targets. One when every target gets the same, as for a producer that
	// is not tied to release lines; several when each line gets its own.
	Releases []Release `json:"releases"`
	Consumer string    `json:"consumer"` // the repository slug
	Holdings []string  `json:"holdings"` // the holdings of the producer in the consumer that are written, by name
	DryRun   bool      `json:"dryRun"`   // the run was a dry run: no branch or PR is created
	// Refresh says --refresh was given, so the report does not suggest it.
	Refresh  bool            `json:"refresh,omitempty"`
	NoPush   bool            `json:"noPush"` // the run stops after the commit, with the branch left in the clone; never set with DryRun
	Status   Status          `json:"status"` // blocked if any target is blocked or failed or the release has a blocker, else ready if any target needs a PR
	Targets  []TargetPlan    `json:"targets"`
	Blockers []string        `json:"blockers,omitempty"` // in the way of the whole release
	Images   []ReleaseImages `json:"images,omitempty"`   // one check for each release that some target needs a PR for
	Warnings []string        `json:"warnings,omitempty"`
	// Companions are the releases that go with the release, because its Artifact
	// pins theirs, each once, in the order the targets need them.
	Companions []Companion `json:"companions,omitempty"`
	// Destination is what the final destination holds of the producer on the lines
	// shipped to, when it holds it and is not the consumer: the reference to compare
	// the release with. A mismatch is a warning of the target.
	Destination []Destination `json:"destination,omitempty"`
	// Notes are lines that were left out of the run, and why, known before any
	// target is read.
	Notes []string `json:"notes,omitempty"`
	// Clone and Remote are the engineer's clone of the consumer and its upstream
	// remote, set by Ship when it made commits, so the report can say how to
	// review them.
	Clone  string `json:"clone,omitempty"`
	Remote string `json:"remote,omitempty"`
	// Push is where the branches are pushed, or would be, set when the run looks
	// at the open PRs.
	Push *PushTarget `json:"push,omitempty"`
	// Actor is who the run acts as, set by Ship before any work.
	Actor *Actor `json:"actor,omitempty"`
}

// ReleaseImages is the check of the images of one release.
type ReleaseImages struct {
	// Producer is the repository of a companion whose release this is, and empty for
	// the release asked for.
	Producer string `json:"producer,omitempty"`
	Release  string `json:"release"` // the tag
	registry.Report
}

// ReleaseOf is the release of a tag the plan ships.
func (p Plan) ReleaseOf(tag string) Release {
	for _, r := range p.Releases {
		if r.Tag == tag {
			return r
		}
	}
	return Release{Tag: tag}
}

// Uniform reports whether every target gets the same release.
func (p Plan) Uniform() bool { return len(p.Releases) <= 1 }

// Check runs the state check for in. It resolves the release, finds the
// consumer, and for each target branch reads what the consumer holds in each
// holding; if any branch needs a PR it checks the release's images exist, once.
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
	sel, err := res.specs(in)
	if err != nil {
		return Plan{}, resolution{}, err
	}
	specs, named := sel.specs, sel.named
	if err := res.route.Ready(); err != nil {
		return Plan{}, resolution{}, err // an edit names a strategy gero does not have yet
	}
	specs, companions := res.withCompanions(ctx, ws, specs)
	plan := Plan{
		Producer: res.producer.Slug(), Consumer: res.route.Consumer.Slug(),
		DryRun: in.DryRun, NoPush: in.NoPush && !in.DryRun, Refresh: in.Refresh,
	}
	for _, h := range res.route.Holdings {
		plan.Holdings = append(plan.Holdings, h.Name())
	}
	plan.Notes = sel.notes
	plan.Companions = companions
	for _, c := range companions {
		if c.Error != "" {
			plan.Blockers = append(plan.Blockers, "companion: "+c.Error)
		}
	}
	if note := skippedNote(res, specs); note != "" {
		plan.Notes = append(plan.Notes, note)
	}
	for _, sp := range specs {
		if !slices.ContainsFunc(plan.Releases, func(r Release) bool { return r.Tag == sp.release.Tag }) {
			plan.Releases = append(plan.Releases, sp.release)
		}
	}
	header := plan
	emit(sink, Event{Kind: EventResolved, Plan: &header})
	destinations, mismatches := res.destinations(ctx, ws, specs)
	var needsPR []string // the tags of the releases that some target needs a PR for
	for _, sp := range specs {
		tp, err := res.checkTarget(ctx, ws, sp, in)
		if err != nil {
			if named {
				return Plan{}, resolution{}, err // asked for this branch, so not being able to read it is the answer
			}
			// One branch of several that could not be read does not stop the
			// others: it is reported as failed.
			tp = TargetPlan{Target: Target{Branch: sp.branch()}, Line: sp.target.Line, Release: sp.release.Tag, Status: StatusFailed, Error: err.Error()}
		}
		tp.Warnings = append(tp.Warnings, mismatches[tp.Line]...)
		if tp.Status == StatusReady && !slices.Contains(needsPR, sp.release.Tag) {
			needsPR = append(needsPR, sp.release.Tag)
		}
		plan.Targets = append(plan.Targets, tp)
		checked := tp
		emit(sink, Event{Kind: EventTargetChecked, Branch: tp.Branch, Line: tp.Line, Target: &checked})
	}
	emit(sink, Event{Kind: EventTargetsChecked})
	plan.Destination = destinations
	for i := range destinations {
		emit(sink, Event{Kind: EventDestination, Line: destinations[i].Line, Destination: &destinations[i]})
	}
	for _, tag := range needsPR {
		images := registry.CheckProject(ctx, res.producer, prober, tag)
		plan.Images = append(plan.Images, ReleaseImages{Release: tag, Report: images})
		plan.Warnings = append(plan.Warnings, images.Warnings...)
		if images.Failed() {
			blocker := "artifacts: the release is not fully published"
			if !plan.Uniform() {
				blocker = "artifacts: " + tag + " is not fully published"
			}
			plan.Blockers = append(plan.Blockers, blocker)
		}
		emit(sink, Event{Kind: EventImagesChecked, Release: tag, Images: &images})
		// The releases that go with this one are looked for right after it, so a
		// report shows them together.
		for _, c := range res.companionsToShip(plan, tag) {
			images := registry.CheckProject(ctx, c.holding.Producer(), prober, c.Tag)
			plan.Images = append(plan.Images, ReleaseImages{Producer: c.Producer, Release: c.Tag, Report: images})
			plan.Warnings = append(plan.Warnings, images.Warnings...)
			if images.Failed() {
				plan.Blockers = append(plan.Blockers, "artifacts: "+c.Tag+" of "+c.Producer+" is not fully published")
			}
			emit(sink, Event{Kind: EventImagesChecked, Producer: c.Producer, Release: c.Tag, Images: &images})
		}
	}
	plan.Status = overall(plan)
	return plan, res, nil
}

// skippedNote says which steps a route that skips the producer's steps leaves out,
// for the targets, or "" when there are none. They are the steps of the edits that
// run from the repo of a project the active team does not own.
func skippedNote(res resolution, specs []spec) string {
	var runs []string
	seen := map[string]bool{}
	for _, sp := range specs {
		for _, ref := range res.route.SkippedSteps(sp.target) {
			key := fmt.Sprintf("%s/%s/%d", ref.Edit, ref.Phase, ref.Index)
			if run := strings.Join(ref.Step.Run, " "); !seen[key] {
				seen[key] = true
				runs = append(runs, "`"+run+"`")
			}
		}
	}
	if len(runs) == 0 {
		return ""
	}
	return fmt.Sprintf("%s is managed by %s, so gero did not run the %d step(s) that run from its repo (%s) and the PRs may be incomplete",
		res.producer.Slug(), orUnspecified(res.producer.ManagedBy()), len(runs), strings.Join(runs, ", "))
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
