package ship

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ResultStatus is what a run did for one target.
type ResultStatus string

// The results.
const (
	// ResultCommitted: the commit is on the new branch in the clone.
	ResultCommitted ResultStatus = "committed"
	// ResultWouldCommit: a dry run made the commit in a worktree and removed it.
	ResultWouldCommit ResultStatus = "would commit"
	// ResultUpToDate: the edit and the steps changed nothing.
	ResultUpToDate ResultStatus = "already up to date"
	// ResultFailed: preparing the branch, a step or the commit failed.
	ResultFailed ResultStatus = "failed"
	// ResultOpened: the branch was pushed and the PR opened.
	ResultOpened ResultStatus = "opened"
	// ResultAlreadyOpen: the actor's own PR at this version is open, so nothing
	// was made.
	ResultAlreadyOpen ResultStatus = "already open"
	// ResultBlocked: the actor's own PR at a newer version is open.
	ResultBlocked ResultStatus = "blocked"
	// ResultRefreshed: the actor's own open PR was rebuilt and its branch replaced.
	ResultRefreshed ResultStatus = "refreshed"
	// ResultWouldRefresh: a dry run found that a refresh would replace the branch.
	ResultWouldRefresh ResultStatus = "would refresh"
)

// TargetResult is what a run did for one target that needed a PR.
type TargetResult struct {
	Status  ResultStatus `json:"status"`
	Branch  string       `json:"branch"`            // the new branch
	Commits []CommitInfo `json:"commits,omitempty"` // the commits made, oldest first
	Place   Place        `json:"place,omitempty"`   // where it was made
	Reason  string       `json:"reason,omitempty"`  // why that was a worktree
	Changes []string     `json:"changes,omitempty"` // what the strategy changed, in words
	Error   string       `json:"error,omitempty"`   // why it failed

	// What publishing did, set only by a run that publishes.
	Pushed     string   `json:"pushed,omitempty"`     // the remote the branch was pushed to
	PR         *PRRef   `json:"pr,omitempty"`         // the PR that was opened
	Superseded []PRRef  `json:"superseded,omitempty"` // the older PRs that were closed
	Warnings   []string `json:"warnings,omitempty"`   // what went wrong that did not fail the target
	// WouldSupersede is what a dry run would have closed.
	WouldSupersede []PRRef `json:"wouldSupersede,omitempty"`
	// BehindBy is how many commits the target has that an already open PR's branch
	// does not, when it is behind.
	BehindBy int `json:"behindBy,omitempty"`
	// Note says more about the result than its status does: that an already open
	// PR is identical to a fresh build, or why it was not refreshed.
	Note string `json:"note,omitempty"`
}

// PRRef is a PR, by number and where to see it.
type PRRef struct {
	Number int    `json:"number"`
	URL    string `json:"url,omitempty"`
}

// Executor is what a run needs to make commits.
type Executor struct {
	Git     Git
	Runner  updatedispatcher.StepRunner
	TempDir string // where worktrees go
	// Publish, when set, finds the open PRs of each target before making anything,
	// and with it a run that is not --no-push or --dry-run pushes and opens the
	// PRs. Without it a run stops at the commits.
	Publish *Publisher
	// Confirm, when set, is asked once before anything is written, if the run is
	// going to publish something: it is given what will be done and says whether
	// to go on. It is not asked for a dry run, a --no-push run, or when nothing
	// would be written. Not set means go on.
	Confirm func(ctx context.Context, c Confirmation) (bool, error)
	// Now is the time, for the line a refresh records. Not set is time.Now.
	Now func() time.Time
}

// Confirmation is what a run is about to do, for the one question before it
// writes: the targets that will get a new PR, the older PRs that will be closed,
// and the targets that will be left alone.
type Confirmation struct {
	Consumer string     // the repository the PRs are in
	Release  string     // the tag being shipped
	Push     PushTarget // where the branches are pushed
	Opens    []string   // the target branches that will get a new PR
	Closes   []PRRef    // the older PRs that will be closed, for every target
	Already  []string   // the target branches whose PR is already open
	Blocked  []string   // the target branches a newer PR of the actor's blocks
	// Refreshes are the target branches whose open PR will be rebuilt, and its
	// branch replaced if it differs: a force push, with a lease.
	Refreshes []string
}

// ErrDeclined is returned by Ship, with the plan, when the confirmation was
// answered no. Nothing was written.
var ErrDeclined = errors.New("declined: nothing was written")

// Tip is the newest commit of the result, or "" when there is none.
func (r TargetResult) Tip() string { return tip(r.Commits) }

// ErrNoRemote is returned by Ship when it is to publish and cannot tell who is
// running, so it does not know whose PRs are its own.
var ErrNoRemote = errors.New("could not tell who is running on GitHub")

// ErrNoClone is returned by Ship when there are commits to make and the
// engineer has no clone of the consumer to make them in.
var ErrNoClone = errors.New("no local clone of the consumer")

// Ship checks the state, as Check does, and then, when ex is not nil and
// nothing blocks the release as a whole, makes the commit for each target that
// needs one: in the engineer's clone or a worktree of it. A dry run makes the
// commit in a worktree and removes it. Nothing is pushed.
//
// A target that fails is reported as failed and the others go on. The plan is
// returned with every error that comes after the check, so it can be reported.
func Ship(ctx context.Context, ws Workspace, prober registry.Prober, in Input, ex *Executor, sink Sink) (Plan, error) {
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
	consumer := plan.Consumer
	clone, err := ws.CloneDir(consumer)
	if err != nil {
		return plan, fmt.Errorf("%w: %v", ErrNoClone, err)
	}
	remote, err := ws.Remote(consumer)
	if err != nil {
		return plan, err
	}
	plan.Clone, plan.Remote = clone, remote
	var pub *Publisher
	var me string
	if ex.Publish != nil && !in.NoPush {
		p := *ex.Publish
		p.Clone = clone
		pub = &p
		consumer := res.route.Consumer
		settings := ws.PushSettings(plan.Consumer)
		target, err := resolvePushTarget(
			PushChoice{Flag: in.PushTo, UserRepo: settings.UserRepo, Project: consumer.PullRequests.PushTo, UserDefault: settings.UserDefault},
			PushRemotes{Fork: settings.Fork, Upstream: settings.Upstream}, in.CI)
		if err != nil {
			return plan, err
		}
		if target.Role == RoleFork && settings.GitHubUser == "" {
			return plan, errors.New("pushing to the fork needs githubUser in the user file: the PR's head is the fork's owner and the branch")
		}
		pub.Push, pub.HeadOwner = target, settings.GitHubUser
		pub.IsBot, pub.BotLabels = in.CI, consumer.PullRequests.BotLabels
		plan.Push = &target
		if me, err = pub.Remote.Me(ctx); err != nil {
			return plan, fmt.Errorf("%w: %v", ErrNoRemote, err)
		}
	}
	// Decide every target first, reading only, so that the one question before
	// anything is written can say what will be done.
	states := make([]targetState, len(ready))
	for k, i := range ready {
		states[k] = res.decideTarget(ctx, pub, me, plan, plan.Targets[i], sink)
	}
	if pub != nil && !in.DryRun && ex.Confirm != nil {
		if c, writes := confirmation(plan, states); writes {
			ok, err := ex.Confirm(ctx, c)
			if err != nil {
				return plan, err
			}
			if !ok {
				return plan, ErrDeclined
			}
		}
	}
	for k, i := range ready {
		plan.Targets[i].Result = res.workTarget(ctx, ex, pub, clone, remote, in, plan, states[k], sink)
		emit(sink, Event{Kind: EventTargetDone, Branch: plan.Targets[i].Branch, Result: plan.Targets[i].Result})
	}
	return plan, nil
}

// targetState is what was found out about one target before any work: the
// decision about its PR, what to warn about, and, for a target that needs no
// work, its result.
type targetState struct {
	tp       TargetPlan
	m        Marker
	dec      Publish
	warnings []string
	done     *TargetResult // set when there is nothing to make: blocked, or the lookup failed
}

// decideTarget looks at the open PRs of a target's branch, when there is a
// publisher, and decides what to do about them. A PR of the actor's own at a newer
// version blocks the target, and a failed lookup fails it, each before any work.
// It only reads.
func (r resolution) decideTarget(ctx context.Context, pub *Publisher, me string, plan Plan, tp TargetPlan, sink Sink) targetState {
	st := targetState{tp: tp, m: Marker{Source: plan.Producer, Consumer: plan.Consumer, Target: tp.Branch, Version: plan.Release.Tag}}
	if pub == nil {
		return st
	}
	open, err := pub.Remote.OpenForBase(ctx, plan.Consumer, tp.Branch, 0)
	if err != nil {
		st.done = &TargetResult{Status: ResultFailed, Branch: tp.Preview.Branch,
			Error: fmt.Sprintf("look for the open PRs of %s: %v", plan.Consumer, err)}
		return st
	}
	st.dec = decidePublish(open.PRs, me, st.m)
	emit(sink, Event{Kind: EventDecided, Branch: tp.Branch, Decision: &st.dec})
	if open.Truncated {
		st.warnings = append(st.warnings, fmt.Sprintf("only the newest %d open PRs of %s were looked at, so an older PR of this bump may have been missed", len(open.PRs), plan.Consumer))
	}
	for _, a := range st.dec.Also {
		st.warnings = append(st.warnings, fmt.Sprintf("also open: #%d by %s (%s)", a.Number, a.Author, a.URL))
	}
	if st.dec.Action == PublishBlocked {
		newer, _ := ParseMarker(st.dec.Newer.Body)
		st.done = &TargetResult{Status: ResultBlocked, Branch: tp.Preview.Branch, Warnings: st.warnings,
			Error: fmt.Sprintf("#%d already ships %s to %s, which is newer than %s", st.dec.Newer.Number, newer.Version, tp.Branch, plan.Release.Tag)}
	}
	return st
}

// confirmation is what the run is about to do, and whether that is anything
// that writes: a PR to open, or an older one to close.
func confirmation(plan Plan, states []targetState) (Confirmation, bool) {
	c := Confirmation{Consumer: plan.Consumer, Release: plan.Release.Tag}
	if plan.Push != nil {
		c.Push = *plan.Push
	}
	for _, st := range states {
		if st.done != nil && st.dec.Action != PublishBlocked {
			continue // the lookup failed: nothing is decided for it
		}
		switch st.dec.Action {
		case PublishBlocked:
			c.Blocked = append(c.Blocked, st.tp.Branch)
		case PublishAlreadyOpen:
			c.Already = append(c.Already, st.tp.Branch)
			c.Closes = append(c.Closes, refs(st.dec.Supersede)...)
		default:
			c.Opens = append(c.Opens, st.tp.Branch)
			c.Closes = append(c.Closes, refs(st.dec.Supersede)...)
		}
	}
	return c, len(c.Opens)+len(c.Closes) > 0
}

// workTarget does the work for one target that was decided. One that needs none
// is its own result. One whose PR is already open only has its older PRs closed
// (or listed, in a dry run). Otherwise it makes the commits and, unless this is a
// dry run, publishes them.
func (r resolution) workTarget(ctx context.Context, ex *Executor, pub *Publisher, clone, remote string, in Input, plan Plan, st targetState, sink Sink) *TargetResult {
	if st.done != nil {
		return st.done
	}
	tp := st.tp
	if pub != nil && st.dec.Action == PublishAlreadyOpen {
		result := &TargetResult{Status: ResultAlreadyOpen, Branch: tp.Preview.Branch, Warnings: st.warnings,
			PR: &PRRef{Number: st.dec.Open.Number, URL: st.dec.Open.URL}}
		if open := st.dec.Open; open.HeadOwner != "" {
			behind, err := pub.Remote.BehindBy(ctx, plan.Consumer, tp.Branch, open.HeadOwner+":"+open.HeadRef)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not tell whether #%d is behind %s: %v", open.Number, tp.Branch, err))
			}
			result.BehindBy = behind
		}
		if in.DryRun {
			result.WouldSupersede = refs(st.dec.Supersede)
		} else {
			pub.supersede(ctx, plan.Consumer, st.dec.Open.Number, st.m, tp.Branch, st.dec.Supersede, result, sink)
		}
		return result
	}
	result := r.commitTarget(ctx, ex, clone, remote, in, tp, sink)
	result.Warnings = append(result.Warnings, st.warnings...)
	switch {
	case pub == nil:
	case in.DryRun:
		result.WouldSupersede = refs(st.dec.Supersede)
	case result.Status == ResultCommitted:
		pub.publish(ctx, plan.Consumer, tp, st.m, st.dec, result, sink)
	}
	return result
}

// refs turns PRs into references.
func refs(prs []pulls.PR) []PRRef {
	out := make([]PRRef, len(prs))
	for i, p := range prs {
		out[i] = PRRef{Number: p.Number, URL: p.URL}
	}
	return out
}

// commitTarget makes the commit for one target and ends the work, whatever
// happened, so the clone is as it was found apart from the branch it keeps.
func (r resolution) commitTarget(ctx context.Context, ex *Executor, clone, remote string, in Input, tp TargetPlan, sink Sink) *TargetResult {
	result := &TargetResult{Branch: tp.Preview.Branch}
	fail := func(err error) *TargetResult {
		result.Status, result.Error = ResultFailed, err.Error()
		return result
	}
	w, err := Prepare(ctx, ex.Git, PrepareRequest{
		CloneDir: clone, Remote: remote, Target: tp.Branch, Base: tp.Commit,
		Branch: tp.Preview.Branch, DryRun: in.DryRun, TempDir: ex.TempDir,
	})
	if errors.Is(err, ErrBaseMoved) {
		return fail(fmt.Errorf("%w; run it again", err))
	}
	if err != nil {
		return fail(err)
	}
	result.Place, result.Reason = w.Place, w.Reason
	emit(sink, Event{Kind: EventTargetStarted, Branch: tp.Branch, Place: w.Place, Reason: w.Reason})
	if w.Reused {
		// The branch is one of ours that is current, so nothing is built: what it
		// holds is what is published.
		result.Commits = w.Existing
		result.Status = ResultCommitted
		if in.DryRun {
			result.Status = ResultWouldCommit
		}
		return result
	}

	applied, err := Apply{
		Invocation: r.invocation, Release: r.rel, Branch: tp.Branch,
		Message: tp.Preview.CommitMessage, Runner: ex.Runner,
		OnCommit:    func(c CommitInfo) { emit(sink, Event{Kind: EventCommit, Branch: tp.Branch, Commit: &c}) },
		StepMessage: stepMessage(previewInput{producer: r.producer, consumer: r.route.Consumer, release: r.release, target: tp.Target}),
	}.Run(ctx, w)
	if errors.Is(err, ErrNothingChanged) {
		result.Status = ResultUpToDate
		return finish(result, w.Finish(ctx, false))
	}
	if err != nil {
		return fail(errors.Join(err, w.Abort(ctx)))
	}
	result.Commits = applied.Commits
	for _, c := range applied.Changes {
		result.Changes = append(result.Changes, c.Description)
	}
	result.Status = ResultCommitted
	if in.DryRun {
		result.Status = ResultWouldCommit
	}
	return finish(result, w.Finish(ctx, !in.DryRun))
}

// finish records a failure to end the work, which is an error even when the
// commit was made.
func finish(result *TargetResult, err error) *TargetResult {
	if err != nil {
		result.Error = "the work was done, but ending it failed: " + err.Error()
		result.Status = ResultFailed
	}
	return result
}
