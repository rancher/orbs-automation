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
		events = append(events, domain.Event{Kind: domain.EventTargetChecked, Branch: t.Branch, Target: &t})
	}
	events = append(events, domain.Event{Kind: domain.EventTargetsChecked})
	if plan.Images != nil {
		events = append(events, domain.Event{Kind: domain.EventImagesChecked, Images: plan.Images})
	}
	for _, t := range plan.Targets {
		r := t.Result
		if r == nil {
			continue
		}
		if r.Place != "" { // a target that failed before a place was chosen never started
			events = append(events, domain.Event{Kind: domain.EventTargetStarted, Branch: t.Branch, Place: r.Place, Reason: r.Reason})
		}
		for i := range r.Commits {
			events = append(events, domain.Event{Kind: domain.EventCommit, Branch: t.Branch, Commit: &r.Commits[i]})
		}
		events = append(events, domain.Event{Kind: domain.EventTargetDone, Branch: t.Branch, Result: r})
	}
	return append(events, domain.Event{Kind: domain.EventDone, Plan: &plan})
}
