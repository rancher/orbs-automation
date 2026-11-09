package configshow

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Run needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	ConfigSource() (workspace.ConfigSource, error)
}

// Run renders the resolved settings in the format s.Output, with the central
// config resolved from the working directory cwd: a local directory, or the
// central bundle, described by its release and age. Config that cannot be found is
// not an error: it is shown as unset.
func Run(s clicfg.Settings, cwd string) (string, error) {
	return run(s, workspace.Open(s, cwd))
}

func run(s clicfg.Settings, ws Workspace) (string, error) {
	src, err := ws.ConfigSource()
	if err != nil && !errors.Is(err, workspace.ErrNotFound) {
		return "", err
	}
	dir := src.Dir
	if src.Bundle != nil {
		// The entry shows where the config is from, and for a bundle that is the
		// release and how old the copy is.
		dir = workspace.CentralConfigDir{Dir: src.Bundle.Describe(), Source: "bundle"}
	}
	return Render(s, dir, s.Output)
}
