package ship

import (
	"context"
	"io"
	"log/slog"

	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// runPlain is runToStyled with no color and no commit preview, which is what
// most tests want.
func runPlain(ctx context.Context, w io.Writer, log *slog.Logger, ws domain.Workspace, prober registry.Prober, ex *domain.Executor, format string, in domain.Input) error {
	return runToStyled(ctx, w, log, ws, prober, ex, format, in, false, style.Plain())
}
