package ship

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/bump"
)

// head is the consumer's branch as the host has it, and its files at that
// commit. It is the first thing read of a target, by a run and by gero repo
// status alike.
func (r resolution) head(ws Reader, branch string) (Target, bump.Tree, error) {
	consumerSlug := r.route.Consumer.Slug()
	head, ok := r.consumerRefs.Branch(branch)
	if !ok {
		return Target{}, nil, fmt.Errorf("%w: %s has no branch %s", ErrBranchNotFound, consumerSlug, branch)
	}
	target := Target{Branch: head.Name, Commit: head.Commit}
	return target, bump.HostTree(ws, consumerSlug, target.Commit), nil
}

// checkTarget reads what one consumer branch holds and decides what a bump of it
// would be: nothing, blocked, or a PR, with the PR previewed. The images are the
// release's and are checked once by the caller. It returns an error when the
// branch or one of its files cannot be read.
func (r resolution) checkTarget(ctx context.Context, ws Reader, sp spec, in Input) (TargetPlan, error) {
	consumerSlug := r.route.Consumer.Slug()
	target, tree, err := r.head(ws, sp.branch())
	if err != nil {
		return TargetPlan{}, err
	}
	checks, err := r.route.Check(ctx, tree, sp.target, sp.release.Tag, sp.release.Commit)
	if err != nil {
		return TargetPlan{}, fmt.Errorf("read %s of %s: %w", target.Branch, consumerSlug, err)
	}

	tp := TargetPlan{Target: target, Line: sp.target.Line, Release: sp.release.Tag, Status: StatusReady}
	var stale []bump.HoldingCheck // the holdings that are not up to date
	var downgrade bool
	for _, c := range checks {
		tp.Holdings = append(tp.Holdings, HoldingPlan{
			Name: c.Holding.Name(), File: c.Change.Path, Current: c.Current, Want: c.Want,
			UpToDate: c.UpToDate, Downgrade: !c.UpToDate && c.Relation == bump.RelationOlder,
		})
		if !c.UpToDate {
			stale = append(stale, c)
			downgrade = downgrade || c.Relation == bump.RelationOlder
		}
	}
	if len(stale) == 0 {
		tp.Status = StatusUpToDate
		return tp, nil
	}
	if downgrade {
		tp.Downgrade = true
		message := "the release is older than what the branch holds"
		if !in.AllowDowngrade {
			// Stop here, as for a branch that is up to date: nothing else is
			// worth showing for a bump that will not happen.
			tp.Status = StatusBlocked
			tp.Blockers = append(tp.Blockers, "downgrade: "+message+"; pass --allow-downgrade to ship it anyway")
			return tp, nil
		}
		tp.Warnings = append(tp.Warnings, "this is a downgrade: "+message)
	}
	pre, post := r.route.Steps(sp.target)
	preview, err := renderPreview(previewInput{
		producer: r.producer, consumer: r.route.Consumer, release: sp.release, target: target,
		checks: stale, steps: append(pre, post...), skipped: r.route.SkippedSteps(sp.target), userPrefix: in.BranchPrefix,
	})
	if err != nil {
		return TargetPlan{}, fmt.Errorf("render the PR: %w", err)
	}
	tp.Preview = &preview
	return tp, nil
}

// targetOf is the engine's target of a planned one: its branch and line, and the
// holdings the plan has for it.
func (r resolution) targetOf(tp TargetPlan) bump.Target {
	t := bump.Target{Branch: tp.Branch, Line: tp.Line}
	for _, hp := range tp.Holdings {
		for _, h := range r.route.Holdings {
			if h.Name() == hp.Name {
				t.Holdings = append(t.Holdings, h)
			}
		}
	}
	return t
}
