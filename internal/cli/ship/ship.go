// Package ship holds the logic of gero ship: shipping a release of a producer
// to one consumer repository. For now it runs the state check and writes
// nothing; the write steps follow (docs/specs/PUSH_TO_REPO.md). It is
// framework-free: cmd/ owns the cobra wiring.
package ship

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
)

// ErrBlocked is returned, with the report still written, when the state check
// finds something in the way, such as an image that is not published.
var ErrBlocked = errors.New("the release cannot be shipped yet")

// Run checks the state of shipping what in names, and renders the plan as
// s.Output (text or json). The plan is returned even with ErrBlocked.
func Run(ctx context.Context, s clicfg.Settings, cwd string, in domain.Input) (string, error) {
	return "", nil
}

func run(ctx context.Context, ws domain.Workspace, prober registry.Prober, format string, in domain.Input) (string, error) {
	return "", nil
}

func renderText(p domain.Plan) string { return "" }

func renderJSON(p domain.Plan) (string, error) { return "", nil }
