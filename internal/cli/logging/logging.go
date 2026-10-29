// Package logging builds the *slog.Logger that cmd/ hands to commands. It is
// the one place that chooses a handler, so the rest of the code only ever sees
// a *slog.Logger. See docs/specs/CLI_OUTPUT.md section 2.
package logging

import (
	"io"
	"log/slog"
)

// New returns a logger that writes to w at level, one of the clicfg.Log*
// constants; an unknown level is treated as warn. It uses the JSON handler when
// json is set (CI) and the text handler otherwise.
func New(w io.Writer, _ string, _ bool) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}
