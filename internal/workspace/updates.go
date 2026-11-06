package workspace

import "github.com/rancher/orbs-automation/internal/updatedispatcher"

// Updates builds the update dispatcher's engine for the central config: every
// update method with its strategy configured, ready to be invoked by a
// producer's edge. A problem that depends on a strategy, such as one that does
// not exist or a `with` value that does not fit it, or an edge whose params do
// not fit the method it names, is returned as the error. The engine is returned
// with it, holding what was fine, so a command that needs one route is not
// stopped by a mistake somewhere else. It returns Config's errors when the
// config itself is unusable.
func (w *Workspace) Updates() (*updatedispatcher.Engine, error) {
	cfg, err := w.Config()
	if err != nil {
		return nil, err
	}
	return updatedispatcher.NewEngine(updatedispatcher.Builtin(), cfg)
}
