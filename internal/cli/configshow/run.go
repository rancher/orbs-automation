package configshow

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/cli/configdir"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Run needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	ConfigDir() (workspace.ConfigDir, error)
}

// Run renders the resolved settings in the format s.Output, with the central
// config directory resolved from the working directory cwd. A directory that
// cannot be found is not an error: it is shown as unset.
func Run(s clicfg.Settings, cwd string) (string, error) {
	return run(s, workspace.Open(s, cwd))
}

func run(s clicfg.Settings, ws Workspace) (string, error) {
	var dir configdir.Result
	d, err := ws.ConfigDir()
	switch {
	case err == nil:
		dir = configdir.Result{Dir: d.Dir, Source: d.Source}
	case !errors.Is(err, workspace.ErrNotFound):
		return "", err
	}
	return Render(s, dir, s.Output)
}
