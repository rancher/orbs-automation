package ship

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
)

// runPlain is runToStyled with no color and no commit preview, which is what
// most tests want.
func runPlain(ctx context.Context, w io.Writer, log *slog.Logger, ws domain.Workspace, prober registry.Prober, ex *proposal.Executor, format string, in domain.Input) error {
	return runToStyled(ctx, w, log, ws, prober, ex, format, in, false, style.Plain())
}

func TestTheRealProbersHaveTheListOfGoReleases(t *testing.T) {
	p := probers()

	assert.NotNil(t, p.Images)
	assert.NotNil(t, p.Modules)
	assert.NotNil(t, p.Go, "so a go.mod with go 1.26 and no toolchain line can be given its newest patch")
}

// opening is the confirmation of a run that opens a PR for each of the branches.
func opening(branches ...string) proposal.Confirmation {
	return proposal.Confirmation{
		Consumer: "rancher/rancher", Release: "v0.5.3",
		Push:  proposal.PushTarget{Role: proposal.RoleFork, Remote: "origin"},
		Opens: branches,
	}
}
