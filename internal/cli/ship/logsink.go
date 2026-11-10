package ship

import (
	"context"
	"log/slog"
	"strings"

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
			slog.String("producer", e.Plan.Producer), slog.String("release", releaseTags(e.Plan)),
			slog.String("consumer", e.Plan.Consumer), slog.Any("holdings", e.Plan.Holdings))
	case domain.EventTargetChecked:
		l.log.LogAttrs(ctx, slog.LevelInfo, "target checked", about(e,
			slog.String("status", string(e.Target.Status)),
			slog.String("current", valuesOf(*e.Target, func(h domain.HoldingPlan) string { return h.Current })),
			slog.String("want", valuesOf(*e.Target, func(h domain.HoldingPlan) string { return h.Want })))...)
	case domain.EventImagesChecked:
		level, failed := slog.LevelInfo, e.Images.Failed()
		if failed {
			level = slog.LevelWarn
		}
		l.log.LogAttrs(ctx, level, "images checked", slog.String("release", e.Release), slog.Int("rows", len(e.Images.Rows)), slog.Bool("failed", failed))
	case domain.EventDestination:
		level := slog.LevelInfo
		attrs := []slog.Attr{slog.String("project", e.Destination.Project), slog.String("line", e.Destination.Line), slog.String("stream", e.Destination.Stream)}
		if e.Destination.Error != "" {
			level = slog.LevelWarn
			attrs = append(attrs, slog.String("error", e.Destination.Error))
		}
		l.log.LogAttrs(ctx, level, "destination read", attrs...)
	case domain.EventTargetStarted:
		attrs := about(e, slog.String("place", string(e.Place)))
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		l.log.LogAttrs(ctx, slog.LevelInfo, "target started", attrs...)
	case domain.EventActor:
		l.log.LogAttrs(ctx, slog.LevelInfo, "acting as",
			slog.String("name", e.Actor.Name), slog.String("email", e.Actor.Email),
			slog.String("login", e.Actor.Login), slog.Bool("signing", e.Actor.Signing))
	case domain.EventDecided:
		level := slog.LevelInfo
		attrs := about(e, slog.String("action", string(e.Decision.Action)), slog.Int("supersede", len(e.Decision.Supersede)))
		if e.Decision.Open != nil {
			attrs = append(attrs, slog.Int("open", e.Decision.Open.Number))
		}
		if e.Decision.Newer != nil {
			level = slog.LevelWarn
			attrs = append(attrs, slog.Int("newer", e.Decision.Newer.Number))
		}
		l.log.LogAttrs(ctx, level, "decided", attrs...)
	case domain.EventPushed:
		l.log.LogAttrs(ctx, slog.LevelInfo, "pushed", about(e, slog.String("remote", e.Remote), slog.String("new_branch", e.NewBranch))...)
	case domain.EventPROpened:
		l.log.LogAttrs(ctx, slog.LevelInfo, "pr opened", about(e, slog.Int("pr", e.PR.Number), slog.String("url", e.PR.URL))...)
	case domain.EventRefreshed:
		l.log.LogAttrs(ctx, slog.LevelInfo, "refreshed", about(e, slog.Int("pr", e.PR.Number), slog.String("url", e.PR.URL))...)
	case domain.EventSuperseded:
		l.log.LogAttrs(ctx, slog.LevelInfo, "superseded", about(e, slog.Int("pr", e.PR.Number))...)
	case domain.EventCommit:
		l.log.LogAttrs(ctx, slog.LevelInfo, "commit", about(e, slog.String("commit", e.Commit.Hash), slog.String("subject", e.Commit.Subject), slog.Int("files", len(e.Commit.Files)))...)
	case domain.EventTargetDone:
		level := slog.LevelInfo
		attrs := about(e, slog.String("status", string(e.Result.Status)), slog.String("new_branch", e.Result.Branch))
		if e.Result.Error != "" {
			level = slog.LevelWarn
			attrs = append(attrs, slog.String("error", e.Result.Error))
		}
		if e.Result.PR != nil {
			attrs = append(attrs, slog.Int("pr", e.Result.PR.Number))
		}
		if len(e.Result.Warnings) > 0 {
			attrs = append(attrs, slog.Int("warnings", len(e.Result.Warnings)))
		}
		l.log.LogAttrs(ctx, level, "target done", attrs...)
	case domain.EventDone:
		l.log.LogAttrs(ctx, slog.LevelInfo, "ship done",
			slog.String("status", string(e.Plan.Status)), slog.Int("blockers", len(e.Plan.Blockers)))
	}
}

// about is the attributes that say which target an event is about: its branch and,
// when it is in one, its Rancher line.
func about(e domain.Event, rest ...slog.Attr) []slog.Attr {
	attrs := []slog.Attr{slog.String("branch", e.Branch)}
	if e.Line != "" {
		attrs = append(attrs, slog.String("line", e.Line))
	}
	return append(attrs, rest...)
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

// releaseTags names the releases of a plan: the tag, or the tags of the several.
func releaseTags(p *domain.Plan) string {
	tags := make([]string, len(p.Releases))
	for i, r := range p.Releases {
		tags[i] = r.Tag
	}
	return strings.Join(tags, ", ")
}
