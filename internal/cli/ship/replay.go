package ship

import domain "github.com/rancher/orbs-automation/internal/ship"

// replay returns the events a run of plan would have emitted, in the order it
// would have emitted them. A finished plan can then be shown by the same code
// that shows a run as it happens, so the two cannot differ.
func replay(plan domain.Plan) []domain.Event {
	header := plan
	header.Targets, header.Images = nil, nil
	events := []domain.Event{{Kind: domain.EventResolved, Plan: &header}}
	for i := range plan.Targets {
		t := plan.Targets[i]
		events = append(events, domain.Event{Kind: domain.EventTargetChecked, Branch: t.Branch, Line: t.Line, Target: &t})
	}
	events = append(events, domain.Event{Kind: domain.EventTargetsChecked})
	for i := range plan.Images {
		events = append(events, domain.Event{Kind: domain.EventImagesChecked, Release: plan.Images[i].Release, Images: &plan.Images[i].Report})
	}
	for _, t := range plan.Targets {
		r := t.Result
		if r == nil {
			continue
		}
		if r.Place != "" { // a target that failed before a place was chosen never started
			events = append(events, domain.Event{Kind: domain.EventTargetStarted, Branch: t.Branch, Line: t.Line, Place: r.Place, Reason: r.Reason})
		}
		for i := range r.Commits {
			events = append(events, domain.Event{Kind: domain.EventCommit, Branch: t.Branch, Line: t.Line, Commit: &r.Commits[i]})
		}
		if r.Pushed != "" {
			events = append(events, domain.Event{Kind: domain.EventPushed, Branch: t.Branch, Line: t.Line, Remote: r.Pushed, NewBranch: r.Branch})
		}
		if r.Status == domain.ResultOpened && r.PR != nil {
			events = append(events, domain.Event{Kind: domain.EventPROpened, Branch: t.Branch, Line: t.Line, PR: r.PR})
		}
		if r.Status == domain.ResultRefreshed {
			events = append(events, domain.Event{Kind: domain.EventRefreshed, Branch: t.Branch, Line: t.Line, PR: r.PR})
		}
		for i := range r.Superseded {
			events = append(events, domain.Event{Kind: domain.EventSuperseded, Branch: t.Branch, Line: t.Line, PR: &r.Superseded[i]})
		}
		events = append(events, domain.Event{Kind: domain.EventTargetDone, Branch: t.Branch, Line: t.Line, Result: r})
	}
	return append(events, domain.Event{Kind: domain.EventDone, Plan: &plan})
}
