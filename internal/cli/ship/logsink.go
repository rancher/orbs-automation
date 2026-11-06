package ship

import (
	"context"
	"log/slog"

	domain "github.com/rancher/orbs-automation/internal/ship"
)

// logSink records the events of a run as structured log records at info, so a
// JSON handler gives machine-readable progress and a text handler the same
// beside the report. It is the logging CLI_OUTPUT describes, and not a second
// mechanism. A branch that failed, and images that were not found, are warnings:
// they are what the reader should act on.
type logSink struct{ log *slog.Logger }

var _ domain.Sink = logSink{}

// Event logs one event. The event that only ends a table has no record.
func (l logSink) Event(e domain.Event) {
	ctx := context.Background()
	switch e.Kind {
	case domain.EventResolved:
		l.log.LogAttrs(ctx, slog.LevelInfo, "ship resolved",
			slog.String("producer", e.Plan.Producer), slog.String("release", e.Plan.Release.Tag),
			slog.String("consumer", e.Plan.Consumer), slog.String("method", e.Plan.Method))
	case domain.EventTargetChecked:
		l.log.LogAttrs(ctx, slog.LevelInfo, "target checked",
			slog.String("branch", e.Branch), slog.String("status", string(e.Target.Status)),
			slog.String("current", e.Target.Current), slog.String("want", e.Target.Want))
	case domain.EventImagesChecked:
		level, failed := slog.LevelInfo, e.Images.Failed()
		if failed {
			level = slog.LevelWarn
		}
		l.log.LogAttrs(ctx, level, "images checked", slog.Int("rows", len(e.Images.Rows)), slog.Bool("failed", failed))
	case domain.EventTargetStarted:
		attrs := []slog.Attr{slog.String("branch", e.Branch), slog.String("place", string(e.Place))}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		l.log.LogAttrs(ctx, slog.LevelInfo, "target started", attrs...)
	case domain.EventCommit:
		l.log.LogAttrs(ctx, slog.LevelInfo, "commit",
			slog.String("branch", e.Branch), slog.String("commit", e.Commit.Hash), slog.String("subject", e.Commit.Subject))
	case domain.EventTargetDone:
		level := slog.LevelInfo
		attrs := []slog.Attr{
			slog.String("branch", e.Branch), slog.String("status", string(e.Result.Status)),
			slog.String("new_branch", e.Result.Branch),
		}
		if e.Result.Error != "" {
			level = slog.LevelWarn
			attrs = append(attrs, slog.String("error", e.Result.Error))
		}
		l.log.LogAttrs(ctx, level, "target done", attrs...)
	case domain.EventDone:
		l.log.LogAttrs(ctx, slog.LevelInfo, "ship done",
			slog.String("status", string(e.Plan.Status)), slog.Int("blockers", len(e.Plan.Blockers)))
	}
}

// sinks sends each event to every sink, in order, skipping the nil ones.
type sinks []domain.Sink

var _ domain.Sink = sinks(nil)

// Event sends e to each sink.
func (s sinks) Event(e domain.Event) {
	for _, sink := range s {
		if sink != nil {
			sink.Event(e)
		}
	}
}
