package workspace

import (
	"github.com/rancher/orbs-automation/internal/bump"
)

// Bump builds the update engine for the central config: every edit with its
// strategy configured, and the routes of release holdings, ready to be shipped
// down. A problem that depends on a strategy, such as one that does not exist or
// a `with` value that does not fit it, or a release holding in a format gero
// cannot write, is returned as the error. The engine is returned with it, holding
// what was fine, so a command that needs one route is not stopped by a mistake
// somewhere else. It returns Chain's errors when the config itself is unusable.
func (w *Workspace) Bump() (*bump.Engine, error) {
	g, err := w.Chain()
	if err != nil {
		return nil, err
	}
	return bump.NewEngine(bump.Builtin(), g)
}
