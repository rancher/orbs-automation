package ship

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// checkTarget reads what one consumer branch holds and decides what a bump of it
// would be: nothing, blocked, or a PR, with the PR previewed. The images are the
// release's and are checked once by the caller. It returns an error when the
// branch or its file cannot be read.
func (r resolution) checkTarget(ctx context.Context, ws Workspace, branch string, in Input) (TargetPlan, error) {
	consumerSlug := r.route.Consumer.Repo.Name
	target, ok := findBranch(r.consumerRefs, branch)
	if !ok {
		return TargetPlan{}, fmt.Errorf("%w: %s has no branch %s", ErrBranchNotFound, consumerSlug, branch)
	}
	chk, err := r.invocation.Check(ctx, remoteTree{ws: ws, slug: consumerSlug, commit: target.Commit}, r.rel)
	if err != nil {
		return TargetPlan{}, fmt.Errorf("read %s of %s: %w", target.Branch, consumerSlug, err)
	}

	tp := TargetPlan{Target: target, Current: chk.Current, Want: chk.Want, File: chk.Change.Path, Status: StatusReady}
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
	var steps []config.Step
	for _, ref := range append(pre, post...) {
		steps = append(steps, ref.Step)
	}
	preview, err := renderPreview(previewInput{
		producer: r.producer, consumer: r.route.Consumer, release: r.release, target: target,
		check: chk, steps: steps, userPrefix: in.BranchPrefix,
	})
	if err != nil {
		return TargetPlan{}, fmt.Errorf("render the PR: %w", err)
	}
	tp.Preview = &preview
	return tp, nil
}

// remoteTree reads files of the consumer at one commit from the host.
type remoteTree struct {
	ws     Workspace
	slug   string
	commit string
}

func (t remoteTree) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return t.ws.UpstreamFile(ctx, t.slug, t.commit, path)
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
