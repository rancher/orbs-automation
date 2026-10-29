// Package logging builds the *slog.Logger that cmd/ hands to commands. It is
// the one place that chooses a handler, so the rest of the code only ever sees
// a *slog.Logger. See docs/specs/CLI_OUTPUT.md section 2.
package logging

import (
	"io"
	"log/slog"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

// New returns a logger that writes to w at level, one of the clicfg.Log*
// constants; an unknown level is treated as warn. It uses the JSON handler when
// json is set (CI) and the text handler otherwise.
func New(w io.Writer, level string, json bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slogLevel(level)}
	if json {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

func slogLevel(level string) slog.Level {
	switch level {
	case clicfg.LogDebug:
		return slog.LevelDebug
	case clicfg.LogInfo:
		return slog.LevelInfo
	case clicfg.LogError:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
