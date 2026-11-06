package ship

import (
	"log/slog"

	domain "github.com/rancher/orbs-automation/internal/ship"
)

// logSink records the events of a run as structured log records at info, so a
// JSON handler gives machine-readable progress and a text handler the same
// beside the report. It is the logging CLI_OUTPUT describes, and not a second
// mechanism.
type logSink struct{ log *slog.Logger }

var _ domain.Sink = logSink{}

// Event logs one event.
//
// STUB: logs nothing until a later commit.
func (logSink) Event(domain.Event) {}

// sinks sends each event to every sink, in order, skipping the nil ones.
type sinks []domain.Sink

var _ domain.Sink = sinks(nil)

// Event sends e to each sink.
//
// STUB: sends nothing until a later commit.
func (sinks) Event(domain.Event) {}
