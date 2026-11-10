// Package graphresolve holds the logic of gero config resolve graph: it draws
// the whole central config as a D2 diagram, each Project a node and each holding
// a node between the Project that produces and the one that holds. It is
// framework-free: cmd/ owns the cobra wiring.
package graphresolve

import (
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Graph needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Chain() (*chain.Graph, error)
}

// Graph draws the central config found from s and the working directory cwd as
// D2 source.
func Graph(s clicfg.Settings, cwd string) (string, error) {
	return graph(workspace.Open(s, cwd))
}

func graph(ws Workspace) (string, error) {
	if _, err := ws.Chain(); err != nil {
		return "", err
	}
	return "", nil
}
