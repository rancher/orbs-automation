package ship

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	// AllowDowngrade ships a release older than what the consumer holds, which is
	// otherwise a blocker.
	AllowDowngrade bool
	// BranchPrefix is the user's branchPrefix setting. When set it replaces the
	// project's whole, and it is empty when the user has none.
	BranchPrefix string
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
	Producer string  `json:"producer"` // the repository slug
	Release  Release `json:"release"`
	Consumer string  `json:"consumer"` // the repository slug
	Strategy string  `json:"strategy"`
	Target   Target  `json:"target"`
	File     string  `json:"file"`
	Current  string  `json:"current"` // what the consumer holds now
	Want     string  `json:"want"`    // what it should hold for the release
	Status   Status  `json:"status"`
	// Downgrade is set when the release is older than what the consumer holds.
	Downgrade bool             `json:"downgrade,omitempty"`
	Blockers  []string         `json:"blockers,omitempty"`
	Images    *registry.Report `json:"images,omitempty"`  // not checked when already up to date
	Preview   *Preview         `json:"preview,omitempty"` // what a run would write; none when already up to date
	Warnings  []string         `json:"warnings,omitempty"`
}

// Check runs the state check for in: it resolves the release, finds the
// consumer and its target branch, reads what the consumer holds now through the
// strategy, and, unless the consumer is already at the release, checks the
// release's images exist. It writes nothing.
func Check(ctx context.Context, ws Workspace, prober registry.Prober, in Input) (Plan, error) {
	producer, consumers, err := ws.Consumers(in.Project)
	if err != nil {
		return Plan{}, err
	}
	consumer, err := pickConsumer(producer, consumers, in.To)
	if err != nil {
		return Plan{}, err
	}
	strategy, err := LookupStrategy(consumer.Source.Update.Strategy)
	if err != nil {
		return Plan{}, err
	}
	consumerSlug := consumer.Project.Repo.Name
	if in.Branch == "" {
		return Plan{}, fmt.Errorf("%w: the release branches of %s are %s",
			ErrBranchRequired, consumerSlug, strings.Join(consumer.Project.ReleaseBranches(), ", "))
	}

	producerRefs, err := ws.UpstreamRefs(ctx, producer.Repo.Name)
	if err != nil {
		return Plan{}, fmt.Errorf("read the tags of %s: %w", producer.Repo.Name, err)
	}
	release, err := ResolveRelease(producer, producerRefs.Tags, in.Arg, in.Prerelease, consumer.Source.Accept.When)
	if err != nil {
		return Plan{}, err
	}

	consumerRefs, err := ws.UpstreamRefs(ctx, consumerSlug)
	if err != nil {
		return Plan{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	target, ok := findBranch(consumerRefs, in.Branch)
	if !ok {
		return Plan{}, fmt.Errorf("%w: %s has no branch %s", ErrBranchNotFound, consumerSlug, in.Branch)
	}

	params := Params(consumer.Source.Update.Params)
	file, err := strategy.File(params)
	if err != nil {
		return Plan{}, fmt.Errorf("strategy %s: %w", consumer.Source.Update.Strategy, err)
	}
	src, err := ws.UpstreamFile(ctx, consumerSlug, target.Commit, file)
	if err != nil {
		return Plan{}, fmt.Errorf("read %s on %s of %s: %w", file, target.Branch, consumerSlug, err)
	}
	current, err := strategy.Read(src, params)
	if err != nil {
		return Plan{}, fmt.Errorf("read %s on %s of %s: %w", file, target.Branch, consumerSlug, err)
	}
	want, err := strategy.Value(params, map[string]string{
		"image":      producer.EffectiveImages()[0].Name,
		"tag":        release.Tag,
		"version":    strings.TrimPrefix(release.Tag, "v"),
		"sourceRepo": producer.Repo.Name,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("strategy %s: %w", consumer.Source.Update.Strategy, err)
	}

	plan := Plan{
		Producer: producer.Repo.Name, Release: release, Consumer: consumerSlug,
		Strategy: consumer.Source.Update.Strategy, Target: target, File: file,
		Current: current, Want: want, Status: StatusReady,
	}
	if current == want {
		plan.Status = StatusUpToDate
		return plan, nil
	}
	preview, err := renderPreview(previewInput{
		producer: producer, consumer: consumer, release: release, target: target,
		strategy: strategy, file: file, current: current, want: want, userPrefix: in.BranchPrefix,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("render the PR: %w", err)
	}
	plan.Preview = &preview
	images := registry.CheckProject(ctx, producer, prober, release.Tag)
	plan.Images, plan.Warnings = &images, images.Warnings
	if images.Failed() {
		plan.Status = StatusBlocked
		plan.Blockers = append(plan.Blockers, "images: the release is not fully published")
	}
	return plan, nil
}

// pickConsumer chooses the consumer to ship to: the one named by to, or the
// only one.
func pickConsumer(producer config.Project, consumers []config.Consumer, to string) (config.Consumer, error) {
	names := make([]string, len(consumers))
	for i, c := range consumers {
		names[i] = c.Project.Name
	}
	if to != "" {
		for _, c := range consumers {
			if strings.EqualFold(c.Project.Name, to) || strings.EqualFold(c.Project.Repo.Name, to) {
				return c, nil
			}
		}
		return config.Consumer{}, fmt.Errorf("%w: %q does not take updates from %s (it is taken by: %s)",
			ErrNoConsumer, to, producer.Name, strings.Join(names, ", "))
	}
	switch len(consumers) {
	case 0:
		return config.Consumer{}, fmt.Errorf("%w: no project takes updates from %s", ErrNoConsumer, producer.Name)
	case 1:
		return consumers[0], nil
	}
	return config.Consumer{}, fmt.Errorf("%w take updates from %s: %s; choose one with --to",
		ErrAmbiguousConsumer, producer.Name, strings.Join(names, ", "))
}

// findBranch returns the branch of the name and the commit at its head.
func findBranch(refs git.Refs, name string) (Target, bool) {
	for _, b := range refs.Branches {
		if b.Name == name {
			return Target{Branch: b.Name, Commit: b.Commit}, true
		}
	}
	return Target{}, false
}
