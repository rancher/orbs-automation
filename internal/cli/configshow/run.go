package configshow

import (
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
	return "", nil
}

func run(s clicfg.Settings, ws Workspace) (string, error) {
	return "", nil
}
