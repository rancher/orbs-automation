package ship

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/semver"
)

// checkTarget reads what one consumer branch holds and decides what a bump of it
// would be: nothing, blocked, or a PR, with the PR previewed. The images are the
// release's and are checked once by the caller. It returns an error when the
// branch or its file cannot be read.
func (r resolution) checkTarget(ctx context.Context, ws Workspace, branch string, in Input) (TargetPlan, error) {
	consumerSlug := r.consumer.Project.Repo.Name
	target, ok := findBranch(r.consumerRefs, branch)
	if !ok {
		return TargetPlan{}, fmt.Errorf("%w: %s has no branch %s", ErrBranchNotFound, consumerSlug, branch)
	}
	src, err := ws.UpstreamFile(ctx, consumerSlug, target.Commit, r.file)
	if err != nil {
		return TargetPlan{}, fmt.Errorf("read %s on %s of %s: %w", r.file, branch, consumerSlug, err)
	}
	current, err := r.strategy.Read(src, r.params)
	if err != nil {
		return TargetPlan{}, fmt.Errorf("read %s on %s of %s: %w", r.file, branch, consumerSlug, err)
	}

	tp := TargetPlan{Target: target, Current: current, Status: StatusReady}
	if current == r.want {
		tp.Status = StatusUpToDate
		return tp, nil
	}
	if downgrade(r.strategy, current, r.want) {
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
	preview, err := renderPreview(previewInput{
		producer: r.producer, consumer: r.consumer, release: r.release, target: target,
		strategy: r.strategy, file: r.file, current: current, want: r.want, userPrefix: in.BranchPrefix,
	})
	if err != nil {
		return TargetPlan{}, fmt.Errorf("render the PR: %w", err)
	}
	tp.Preview = &preview
	return tp, nil
}

// downgrade reports whether want is an older version than current, by what the
// strategy finds in each. A value with no version in it is not judged.
func downgrade(s Strategy, current, want string) bool {
	cv, ok := s.Version(current)
	if !ok {
		return false
	}
	wv, ok := s.Version(want)
	return ok && semver.Compare(cv, wv) > 0
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
