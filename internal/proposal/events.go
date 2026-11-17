package proposal

// EventKind says what happened.
type EventKind string

// The events a run reports as it goes. The kinds are the words a command's report and
// log know them by.
const (
	// EventPushed: the branch is on the remote, named in Remote.
	EventPushed EventKind = "pushed"
	// EventPROpened: the PR was opened. PR is it.
	EventPROpened EventKind = "pr opened"
	// EventSuperseded: an older PR was commented on and closed. PR is it.
	EventSuperseded EventKind = "superseded"
)

// Event is one thing that happened in a run, with what belongs to it. Only the fields
// of its Kind are set.
type Event struct {
	Kind   EventKind
	Branch string // the base branch, for the events about one
	// Line is the Rancher line of that branch, when it is in one.
	Line string
	// NewBranch is the branch that was made, for Pushed.
	NewBranch string
	Remote    string // Pushed: the remote's name
	PR        *PRRef // PROpened, Superseded
}

// Sink receives the events of a run as they happen, on the goroutine that runs it. A nil
// Sink is allowed and means none.
type Sink interface {
	Event(Event)
}

// emit sends e to the sink, if there is one.
func emit(sink Sink, e Event) {
	if sink != nil {
		sink.Event(e)
	}
}
