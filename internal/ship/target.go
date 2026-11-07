package ship

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// head is the consumer's branch as the host has it, and its files at that
// commit. It is the first thing read of a target, by a run and by gero repo
// status alike.
func (r resolution) head(ws Reader, branch string) (Target, updatedispatcher.Tree, error) {
	consumerSlug := r.route.Consumer.Repo.Name
	head, ok := r.consumerRefs.Branch(branch)
	if !ok {
		return Target{}, nil, fmt.Errorf("%w: %s has no branch %s", ErrBranchNotFound, consumerSlug, branch)
	}
	target := Target{Branch: head.Name, Commit: head.Commit}
	return target, updatedispatcher.HostTree(ws, consumerSlug, target.Commit), nil
}

// checkTarget reads what one consumer branch holds and decides what a bump of it
// would be: nothing, blocked, or a PR, with the PR previewed. The images are the
// release's and are checked once by the caller. It returns an error when the
// branch or its file cannot be read.
func (r resolution) checkTarget(ctx context.Context, ws Reader, sp spec, in Input) (TargetPlan, error) {
	branch := sp.branch
	consumerSlug := r.route.Consumer.Repo.Name
	target, tree, err := r.head(ws, branch)
	if err != nil {
		return TargetPlan{}, err
	}
	chk, err := r.invocation.Check(ctx, tree, r.relOf(sp.release))
	if err != nil {
		return TargetPlan{}, fmt.Errorf("read %s of %s: %w", target.Branch, consumerSlug, err)
	}

	tp := TargetPlan{Target: target, Line: r.route.Consumer.LineOfBranch(branch), Release: sp.release.Tag, Current: chk.Current, Want: chk.Want, File: chk.Change.Path, Status: StatusReady}
	if chk.UpToDate {
		tp.Status = StatusUpToDate
		return tp, nil
	}
	if chk.Relation == updatedispatcher.RelationOlder {
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
	pre, post := r.invocation.Steps(target.Branch)
	preview, err := renderPreview(previewInput{
		producer: r.producer, consumer: r.route.Consumer, release: sp.release, target: target,
		check: chk, steps: append(pre, post...), userPrefix: in.BranchPrefix,
	})
	if err != nil {
		return TargetPlan{}, fmt.Errorf("render the PR: %w", err)
	}
	tp.Preview = &preview
	return tp, nil
}
