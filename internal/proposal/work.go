package proposal

import (
	"context"
	"errors"
	"fmt"
)

// CommitInput is one proposal to build: where, in which branch, and how to write it.
type CommitInput struct {
	Clone, Remote string
	Subject       Subject
	DryRun        bool
	// CI says the run owns its checkout, so what an earlier step left in it is the work.
	CI bool
	// Apply writes the change in the prepared work directory and makes its commits, calling
	// onCommit for each as it is made. A run in which nothing changed is ErrNothingChanged.
	Apply func(w *Work, onCommit func(CommitInfo)) (ApplyResult, error)
}

// Commit makes the commits for one proposal and ends the work, whatever happened, so the
// clone is as it was found apart from the branch it keeps. The result says what was done.
func Commit(ctx context.Context, ex *Executor, in CommitInput, sink Sink) *TargetResult {
	s := in.Subject
	result := &TargetResult{Branch: s.Preview.Branch}
	fail := func(err error) *TargetResult {
		result.Status, result.Error = ResultFailed, err.Error()
		return result
	}
	w, err := Prepare(ctx, ex.Git, PrepareRequest{
		CloneDir: in.Clone, Remote: in.Remote, Target: s.Branch, Base: s.Commit,
		Branch: s.Preview.Branch, DryRun: in.DryRun, TempDir: ex.TempDir,
		StageUntracked: in.CI,
	})
	if errors.Is(err, ErrBaseMoved) {
		return fail(fmt.Errorf("%w; run it again", err))
	}
	if err != nil {
		return fail(err)
	}
	result.Place, result.Reason = w.Place, w.Reason
	emit(sink, Event{Kind: EventTargetStarted, Branch: s.Branch, Line: s.Line, Place: w.Place, Reason: w.Reason})
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
	applied, err := in.Apply(w, func(c CommitInfo) {
		emit(sink, Event{Kind: EventCommit, Branch: s.Branch, Line: s.Line, Commit: &c})
	})
	if errors.Is(err, ErrNothingChanged) {
		result.Status = ResultUpToDate
		return Finish(result, w.Finish(ctx, false))
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
	return Finish(result, w.Finish(ctx, !in.DryRun))
}

// EnactInput is one proposal that was decided, and what is needed to carry it out.
type EnactInput struct {
	Repo    string   // the repository the PR is in, owner/name
	Session *Session // the clone, the remote and the publisher of the run
	Subject Subject
	Marker  Marker
	Decided Decided
	DryRun  bool
	CI      bool
	// Apply writes the change; see CommitInput.
	Apply func(w *Work, onCommit func(CommitInfo)) (ApplyResult, error)
	// Refresh, when set, is called for a proposal whose PR is already open, to rebuild it and
	// replace its branch if it differs: the actor wanted that and the PR is the actor's own.
	Refresh func(result *TargetResult)
}

// Enact does the work for one proposal that was decided. One that needs none is its own
// result. One whose PR is already open only has its older PRs closed (or listed, in a dry
// run). Otherwise it makes the commits and, unless this is a dry run, publishes them.
func Enact(ctx context.Context, ex *Executor, in EnactInput, sink Sink) *TargetResult {
	st, pub, s := in.Decided, in.Session.Pub, in.Subject
	if st.Done != nil {
		return st.Done
	}
	if pub != nil && st.Decision.Action == PublishAlreadyOpen {
		result := &TargetResult{Status: ResultAlreadyOpen, Branch: s.Preview.Branch, Warnings: st.Warnings,
			PR: &PRRef{Number: st.Decision.Open.Number, URL: st.Decision.Open.URL}}
		if open := st.Decision.Open; open.HeadOwner != "" {
			behind, err := pub.Remote.BehindBy(ctx, in.Repo, s.Branch, open.HeadOwner+":"+open.HeadRef)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not tell whether #%d is behind %s: %v", open.Number, s.Branch, err))
			}
			result.BehindBy = behind
		}
		if in.DryRun {
			result.WouldSupersede = Refs(st.Decision.Supersede)
		} else {
			pub.Supersede(ctx, in.Repo, st.Decision.Open.Number, in.Marker, s, st.Decision.Supersede, result, sink)
		}
		if in.Refresh != nil {
			in.Refresh(result)
		}
		return result
	}
	result := Commit(ctx, ex, CommitInput{
		Clone: in.Session.Clone, Remote: in.Session.Remote, Subject: s, DryRun: in.DryRun, CI: in.CI, Apply: in.Apply,
	}, sink)
	result.Warnings = append(result.Warnings, st.Warnings...)
	switch {
	case pub == nil:
	case in.DryRun:
		result.WouldSupersede = Refs(st.Decision.Supersede)
	case result.Status == ResultCommitted:
		pub.Publish(ctx, in.Repo, s, in.Marker, st.Decision, result, sink)
	}
	return result
}
