package proposal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// recording is the events of a run, as a Sink.
type recording struct{ events []Event }

func (r *recording) Event(e Event) { r.events = append(r.events, e) }

func (r *recording) kinds() []EventKind {
	out := make([]EventKind, len(r.events))
	for i, e := range r.events {
		out[i] = e.Kind
	}
	return out
}

// of returns the events of a kind.
func (r *recording) of(kind EventKind) []Event {
	var out []Event
	for _, e := range r.events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// first is the first event of a kind, which the test requires.
func (r *recording) first(t *testing.T, kind EventKind) Event {
	t.Helper()
	events := r.of(kind)
	require.NotEmpty(t, events, "there is a %q event in %v", kind, r.kinds())
	return events[0]
}
