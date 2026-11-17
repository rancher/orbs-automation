package ship

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/gero/authorship"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/registry"
)

// Ship checks the state, as Check does, and then, when ex is not nil and
// nothing blocks the release as a whole, makes the commit for each target that
// needs one: in the engineer's clone or a worktree of it. A dry run makes the
// commit in a worktree and removes it. Nothing is pushed.
//
// A target that fails is reported as failed and the others go on. The plan is
// returned with every error that comes after the check, so it can be reported.
func Ship(ctx context.Context, ws Workspace, prober registry.Prober, in Input, ex *proposal.Executor, sink Sink) (Plan, error) {
	plan, res, err := check(ctx, ws, prober, in, sink)
	if err != nil {
		return plan, err
	}
	// The run is over whichever way it ends from here, and the plan is whole.
	defer func() { emit(sink, Event{Kind: EventDone, Plan: &plan}) }()
	if ex == nil || len(plan.Blockers) > 0 {
		return plan, nil
	}
	var ready []int
	for i, tp := range plan.Targets {
		if tp.Status == StatusReady && tp.Preview != nil {
			ready = append(ready, i)
		}
	}
	if len(ready) == 0 {
		return plan, nil
	}
	sess, err := proposal.Begin(ctx, ws, ex, proposal.SessionInput{
		Repo: res.route.Consumer, PushTo: in.PushTo, CI: in.CI, Refresh: in.Refresh, NoPush: in.NoPush,
	})
	if err != nil {
		return plan, err
	}
	plan.Clone, plan.Remote, plan.Push, plan.Actor = sess.Clone, sess.Remote, sess.Push, sess.Actor
	if sess.Actor != nil {
		emit(sink, Event{Kind: EventActor, Actor: sess.Actor})
	}
	res.committer = sess.Committer
	pub, me := sess.Pub, sess.Me
	// Decide every target first, reading only, so that the one question before
	// anything is written can say what will be done.
	states := make([]targetState, len(ready))
	for k, i := range ready {
		states[k] = res.decideTarget(ctx, pub, me, plan, plan.Targets[i], sink)
	}
	c, writes := confirmation(plan, states, in.Refresh)
	if err := sess.Confirm(ctx, ex, in.DryRun, c, writes); err != nil {
		return plan, err
	}
	for k, i := range ready {
		plan.Targets[i].Result = res.workTarget(ctx, ex, sess, in, plan, states[k], sink)
		emit(sink, Event{Kind: EventTargetDone, Branch: plan.Targets[i].Branch, Line: plan.Targets[i].Line, Result: plan.Targets[i].Result})
	}
	return plan, nil
}

// targetState is what was found out about one target before any work: the
// decision about its PR, what to warn about, and, for a target that needs no
// work, its result.
type targetState struct {
	tp       TargetPlan
	m        proposal.Marker
	dec      proposal.Decision
	warnings []string
	done     *proposal.TargetResult // set when there is nothing to make: blocked, or the lookup failed
}

// decideTarget looks at the open PRs of a target's branch, when there is a
// publisher, and decides what to do about them. A PR of the actor's own at a newer
// version blocks the target, and a failed lookup fails it, each before any work.
// It only reads.
func (r resolution) decideTarget(ctx context.Context, pub *proposal.Publisher, me string, plan Plan, tp TargetPlan, sink Sink) targetState {
	st := targetState{tp: tp, m: proposal.Marker{Source: plan.Producer, Consumer: plan.Consumer, Target: tp.Branch, Version: tp.Release}}
	d := proposal.Decide(ctx, pub, me, proposal.DecideInput{
		Repo: plan.Consumer, Subject: tp.Subject(), Marker: st.m, Branches: r.consumerRefs.Branches,
		Blocked: func(newer proposal.Marker, p pulls.PR) string {
			return fmt.Sprintf("#%d already ships %s to %s, which is newer than %s", p.Number, newer.Version, tp.Branch, tp.Release)
		},
	}, asProposalSink(sink))
	st.dec, st.warnings, st.done = d.Decision, d.Warnings, d.Done
	return st
}

// confirmation is what the run is about to do, and whether that is anything
// that writes: a PR to open, or an older one to close.
func confirmation(plan Plan, states []targetState, refresh bool) (proposal.Confirmation, bool) {
	c := proposal.Confirmation{Consumer: plan.Consumer, Release: releaseTags(plan)}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	for _, st := range states {
		if st.done != nil && st.dec.Action != proposal.PublishBlocked {
			continue // the lookup failed: nothing is decided for it
		}
		switch st.dec.Action {
		case proposal.PublishBlocked:
			c.Blocked = append(c.Blocked, st.tp.Key())
		case proposal.PublishAlreadyOpen:
			if refresh {
				c.Refreshes = append(c.Refreshes, st.tp.Key())
			} else {
				c.Already = append(c.Already, st.tp.Key())
			}
			c.Closes = append(c.Closes, proposal.Refs(st.dec.Supersede)...)
		default:
			c.Opens = append(c.Opens, st.tp.Key())
			c.Closes = append(c.Closes, proposal.Refs(st.dec.Supersede)...)
		}
	}
	c.MayReplace = refresh && len(c.Opens) > 0
	return c, len(c.Opens)+len(c.Closes)+len(c.Refreshes) > 0
}

// workTarget does the work for one target that was decided, as the conveyor does for any
// proposal: the write of the release in the consumer's holdings and the steps of the edits,
// then, with a publisher, the push and the PR.
func (r resolution) workTarget(ctx context.Context, ex *proposal.Executor, sess *proposal.Session, in Input, plan Plan, st targetState, sink Sink) *proposal.TargetResult {
	tp := st.tp
	var refresh func(*proposal.TargetResult)
	if in.Refresh {
		refresh = func(result *proposal.TargetResult) {
			r.refresh(ctx, ex, sess.Pub, sess.Clone, sess.Remote, in, plan, st, result, sink)
		}
	}
	return proposal.Enact(ctx, ex, proposal.EnactInput{
		Repo: plan.Consumer, Session: sess, Subject: tp.Subject(), Marker: st.m, DryRun: in.DryRun, CI: in.CI,
		Decided: proposal.Decided{Decision: st.dec, Warnings: st.warnings, Done: st.done},
		Apply:   r.applier(ctx, ex, tp, plan.ReleaseOf(tp.Release)),
		Refresh: refresh,
	}, asProposalSink(sink))
}

// applier is the write of a release into a target's branch, as the conveyor runs it: the
// holdings of the target, then the steps of the edits, each commit made as the run's
// committer.
func (r resolution) applier(ctx context.Context, ex *proposal.Executor, tp TargetPlan, release Release) func(*proposal.Work, func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
	pv := previewInput{producer: r.producer, consumer: r.route.Consumer, release: release, target: tp.Target}
	return func(w *proposal.Work, onCommit func(proposal.CommitInfo)) (proposal.ApplyResult, error) {
		return proposal.Apply{
			Route: r.route, Target: r.targetOf(tp), Tag: release.Tag, Commit: release.Commit,
			Message: tp.Preview.CommitMessage, Runner: ex.Runner, Sign: ex.Sign,
			Name: r.committer.Name, Email: r.committer.Email, OnCommit: onCommit,
			StepMessage: stepMessage(pv), StrategyMessage: strategyMessage(pv),
		}.Run(ctx, w)
	}
}

// refresh rebuilds an already open PR of the actor's and replaces its branch when
// that would change it. It only does so for a branch that is the actor's to
// change: the PR's head is in the repository this run pushes to, under the name
// this run would use, and every commit on it is ours (authorship.AreOurs). Otherwise
// result.Note says which condition failed, and nothing is changed.
//
// The new branch is built from the current head of the target on a scratch
// branch, and compared with the old one by the change each makes
// (Git.ChangeFingerprint), so the existing branch is not touched until it is known
// to differ. When they are the same, even if the old one is behind, nothing is
// pushed. When they differ the branch is replaced with a push that is only allowed
// while the remote's branch is still where it was seen, the PR's title and body
// are refreshed, and the refresh is recorded in the one comment gero keeps on the
// PR. A dry run does everything but the replacing and says it would.
func (r resolution) refresh(ctx context.Context, ex *proposal.Executor, pub *proposal.Publisher, clone, remote string, in Input, plan Plan, st targetState, result *proposal.TargetResult, sink Sink) {
	tp, open, branch, g := st.tp, st.dec.Open, st.tp.Preview.Branch, ex.Git
	skip := func(format string, args ...any) { result.Note = "not refreshed: " + fmt.Sprintf(format, args...) }
	fail := func(format string, args ...any) {
		result.Status, result.Error = proposal.ResultFailed, fmt.Sprintf(format, args...)
	}

	if want := pub.HeadOwnerOf(plan.Consumer); open.HeadOwner != want {
		skip("its branch is in %s's repository, and this run pushes to %s's", open.HeadOwner, want)
		return
	}
	if open.HeadRef != branch {
		skip("its branch is %s, and this run would use %s", open.HeadRef, branch)
		return
	}
	base, err := g.Fetch(ctx, clone, remote, tp.Branch)
	if err != nil {
		skip("could not read %s: %v", tp.Branch, err)
		return
	}
	if base != tp.Commit {
		skip("%s moved since it was checked; run it again", tp.Branch)
		return
	}
	tip, err := g.Fetch(ctx, clone, pub.Push.Remote, branch)
	if err != nil {
		skip("could not read %s on %s: %v", branch, pub.Push.Remote, err)
		return
	}
	mergeBase, err := g.MergeBase(ctx, clone, base, tip)
	if err != nil {
		skip("%v", err)
		return
	}
	facts, err := g.CommitsSince(ctx, clone, mergeBase, tip)
	if err != nil {
		skip("%v", err)
		return
	}
	email, err := g.Config(ctx, clone, "user.email")
	if err != nil {
		skip("%v", err)
		return
	}
	if ours, why := authorship.AreOurs(facts, email); !ours {
		skip("%s", why)
		return
	}
	before, err := g.ChangeFingerprint(ctx, clone, mergeBase, tip)
	if err != nil {
		skip("%v", err)
		return
	}

	scratchName := branch + ".refresh"
	scratch := tp
	preview := *tp.Preview
	preview.Branch = scratchName
	scratch.Preview = &preview
	built := proposal.Commit(ctx, ex, proposal.CommitInput{
		Clone: clone, Remote: remote, Subject: scratch.Subject(), DryRun: in.DryRun, CI: in.CI,
		Apply: r.applier(ctx, ex, scratch, plan.ReleaseOf(tp.Release)),
	}, asProposalSink(sink))
	switch built.Status {
	case proposal.ResultCommitted, proposal.ResultWouldCommit:
	case proposal.ResultUpToDate:
		skip("a fresh build changes nothing, so there is nothing to put on the branch")
		return
	default:
		fail("rebuild %s: %s", branch, built.Error)
		return
	}
	after, err := g.ChangeFingerprint(ctx, clone, base, built.Tip())
	if err != nil {
		fail("compare the rebuilt branch with %s: %v", branch, err)
		return
	}
	now := time.Now
	if ex.Now != nil {
		now = ex.Now
	}
	stamp := now().UTC().Format("2006-01-02 15:04 UTC")
	behind := result.BehindBy > 0
	if before == after && !behind {
		if !in.DryRun {
			if err := g.DeleteBranch(ctx, clone, scratchName); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove the scratch branch %s: %v", scratchName, err))
			}
		}
		result.Note = "identical to a fresh build"
		switch _, stale := proposal.DescriptionStale(*open, tp.Subject()); {
		case !stale:
		case in.DryRun:
			result.Note += "; would update the description"
		case pub.UpdateDescription(ctx, plan.Consumer, *open, tp.Subject(), result):
			result.Note += "; description updated"
			pub.RecordRefresh(ctx, plan.Consumer, open.Number, "- Updated the description ("+stamp+")", result)
		}
		return
	}
	result.Commits, result.Place, result.Reason = built.Commits, built.Place, built.Reason
	if in.DryRun {
		result.Status = proposal.ResultWouldRefresh
		return
	}
	// The commit is pushed as it is and not by the name of the local branch, which
	// may be checked out in the engineer's clone and so cannot be moved.
	if err := g.PushCommitReplacing(ctx, clone, pub.Push.Remote, branch, built.Tip(), tip); err != nil {
		fail("replace %s on %s: %v", branch, pub.Push.Remote, err)
		return
	}
	if err := g.ForceBranch(ctx, clone, branch, built.Tip()); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("left %s in your clone as it was, since it is checked out or could not be moved: %v", branch, err))
	}
	if err := g.DeleteBranch(ctx, clone, scratchName); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove the scratch branch %s: %v", scratchName, err))
	}
	result.Status, result.Pushed = proposal.ResultRefreshed, pub.Push.Remote
	emit(sink, Event{Kind: EventPushed, Branch: tp.Branch, Line: tp.Line, Remote: pub.Push.Remote, NewBranch: branch})
	emit(sink, Event{Kind: EventRefreshed, Branch: tp.Branch, Line: tp.Line, PR: result.PR})
	pub.UpdateDescription(ctx, plan.Consumer, *open, tp.Subject(), result)
	line := fmt.Sprintf("- Rebuilt on %s@%s (%s)", tp.Branch, proposal.Short(base), stamp)
	if behind {
		line += fmt.Sprintf(", was %d behind", result.BehindBy)
	}
	pub.RecordRefresh(ctx, plan.Consumer, open.Number, line, result)
}

// releaseTags names the releases of a plan: the one tag, or the tags of the
// several, in order.
func releaseTags(p Plan) string {
	tags := make([]string, len(p.Releases))
	for i, r := range p.Releases {
		tags[i] = r.Tag
	}
	return strings.Join(tags, ", ")
}
