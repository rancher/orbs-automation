// Package ciship holds the logic of gero ci:ship: the same shipping as gero
// ship, run by a workflow. It is never interactive, and it refuses to start
// without a token that reaches the consumer (docs/specs/CI_SHIP.md).
package ciship

import (
	"context"
	"io"
	"log/slog"

	"github.com/rancher/orbs-automation/internal/cli/cigate"
	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
)

// run is what Run hands the work to; a test replaces it.
var run = ship.Run

// Run ships what opts names, as a workflow does: it asks nothing, and it pushes
// and opens the PRs, so a token that only reaches the workflow's own repository
// is refused before anything is written.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts ship.Options) error {
	if err := cigate.RequireCrossRepoToken("gero ci:ship", s); err != nil {
		return err
	}
	opts.Yes, opts.Interactive, opts.In = true, false, nil
	opts.NoPush = false
	return run(ctx, w, log, s, cwd, opts)
}
