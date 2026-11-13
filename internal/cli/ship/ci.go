package ship

import (
	"context"
	"io"
	"log/slog"

	"github.com/rancher/orbs-automation/internal/cli/cigate"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
)

// runShip is what RunCI hands the work to; a test replaces it.
var runShip = Run

// RunCI is gero ci:ship: Run as a workflow does it, never interactive and
// refusing to start without a token that reaches the consumer
// (docs/specs/CI_SHIP.md). It ships what opts names: it asks nothing, and it pushes
// and opens the PRs, so a token that only reaches the workflow's own repository
// is refused before anything is written.
func RunCI(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts Options) error {
	if err := cigate.RequireCrossRepoToken("gero ci:ship", s); err != nil {
		return err
	}
	opts.Yes, opts.Interactive, opts.In = true, false, nil
	opts.NoPush = false
	return runShip(ctx, w, log, s, cwd, opts)
}
