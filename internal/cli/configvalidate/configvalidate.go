// Package configvalidate holds the logic of gero config validate: it loads the
// central application config and reports whether it is usable. It is
// framework-free: cmd/ owns the cobra wiring.
package configvalidate

import (
	"fmt"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Run needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	ConfigDir() (workspace.ConfigDir, error)
	Config() (config.Config, error)
	// Updates builds the update dispatcher's engine, which finds what depends on
	// a strategy: a method that cannot work, or an edge that does not fit its method.
	Updates() (*updatedispatcher.Engine, error)
}

// Run validates the central config found from s and the working directory cwd.
// It returns a one-line summary on success, and on failure the error naming
// every problem, or workspace.ErrNotFound when there is no config directory.
func Run(s clicfg.Settings, cwd string) (string, error) {
	return run(workspace.Open(s, cwd), style.New(s.Terminal))
}

func run(ws Workspace, view *style.Renderer) (string, error) {
	dir, err := ws.ConfigDir()
	if err != nil {
		return "", err
	}
	cfg, err := ws.Config()
	if err != nil {
		return "", err
	}
	if _, err := ws.Updates(); err != nil {
		return "", err
	}
	lines := 0
	if cfg.Lines != nil {
		lines = len(cfg.Lines.Lines)
	}
	return fmt.Sprintf("%s: %s (projects: %d, externals: %d, lines: %d)\n",
		view.Style(style.OK, "config OK"), dir.Dir, len(cfg.Projects), len(cfg.Externals), lines), nil
}
