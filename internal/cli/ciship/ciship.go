// Package ciship holds the logic of gero ci:ship: the same shipping as gero
// ship, run by a workflow. It is never interactive, and it refuses to start
// without a token that reaches the consumer (docs/specs/CI_SHIP.md).
package ciship

import (
	"context"
	"io"
	"log/slog"

	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/gero/errs"
)

// run is what Run hands the work to; a test replaces it.
var run = ship.Run

// Run ships what opts names, as a workflow does.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts ship.Options) error {
	_ = run
	return errs.NotImplemented("gero ci:ship", "")
}

var _ = io.Discard
