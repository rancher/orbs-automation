package ship

import "github.com/rancher/orbs-automation/internal/registry"

// EventKind says what happened.
type EventKind string

// The events of a run, in the order they happen.
const (
	// EventResolved: the release and the route are known. Plan has the header
	// (producer, release, consumer, method, file) and no targets yet.
	EventResolved EventKind = "resolved"
	// EventTargetChecked: one branch has been read. Target is its state.
	EventTargetChecked EventKind = "target checked"
	// EventTargetsChecked: every branch has been read, so what they hold can be
	// shown before the slower checks that follow.
	EventTargetsChecked EventKind = "targets checked"
	// EventImagesChecked: the release's images have been looked for, once.
	EventImagesChecked EventKind = "images checked"
	// EventTargetStarted: the work for one branch has begun, in Place.
	EventTargetStarted EventKind = "target started"
	// EventCommit: a commit was made for the branch. Commit is it.
	EventCommit EventKind = "commit"
	// EventDecided: what to do about the PR of a branch is known: open one, one
	// is already open, or a newer one blocks it. Decision is it.
	EventDecided EventKind = "decided"
	// EventPushed: the branch is on the remote, named in Remote.
	EventPushed EventKind = "pushed"
	// EventPROpened: the PR was opened. PR is it.
	EventPROpened EventKind = "pr opened"
	// EventSuperseded: an older PR was commented on and closed. PR is it.
	EventSuperseded EventKind = "superseded"
	// EventTargetDone: the work for one branch has ended, with Result.
	EventTargetDone EventKind = "target done"
	// EventDone: the run has ended. Plan is the whole of it.
	EventDone EventKind = "done"
)

// Event is one thing that happened in a run, with what belongs to it. Only the
// fields of its Kind are set.
type Event struct {
	Kind   EventKind
	Branch string // the consumer branch, for the events about one

	Plan     *Plan            // Resolved: the header. Done: everything
	Target   *TargetPlan      // TargetChecked
	Images   *registry.Report // ImagesChecked
	Place    Place            // TargetStarted: where the work is done
	Reason   string           // TargetStarted: why that was a worktree
	Commit   *CommitInfo      // Commit
	Decision *Publish         // Decided
	Remote   string           // Pushed: the remote's name
	PR       *PRRef           // PROpened, Superseded
	Result   *TargetResult    // TargetDone
}

// Sink receives the events of a run as they happen, on the goroutine that runs
// it, so it can print them. A nil Sink is allowed and means none.
type Sink interface {
	Event(Event)
}

// emit sends e to the sink, if there is one.
func emit(sink Sink, e Event) {
	if sink != nil {
		sink.Event(e)
	}
}
