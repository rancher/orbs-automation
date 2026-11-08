// Package configvalidate holds the logic of gero config validate: it loads the
// central application config and reports whether it is usable. It is
// framework-free: cmd/ owns the cobra wiring.
package configvalidate

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Run needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	CentralConfigDir() (workspace.CentralConfigDir, error)
	Chain() (*chain.Graph, error)
}

// Run validates the central config found from s and the working directory cwd.
// It returns a summary on success, with a line for each warning, and on failure
// the error naming every problem, or workspace.ErrNotFound when there is no
// config directory.
func Run(s clicfg.Settings, cwd string) (string, error) {
	return run(workspace.Open(s, cwd), style.New(s.Terminal))
}

func run(ws Workspace, view *style.Renderer) (string, error) {
	dir, err := ws.CentralConfigDir()
	if err != nil {
		return "", err
	}
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	var streams, artifacts int
	for _, p := range g.Projects() {
		streams += len(p.Streams())
		artifacts += len(p.Artifacts())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (projects: %d, streams: %d, artifacts: %d, holdings: %d, lines: %d)\n",
		view.Style(style.OK, "config OK"), dir.Dir, len(g.Projects()), streams, artifacts, len(g.Holdings()), len(g.Lines()))
	for _, w := range g.Warnings() {
		fmt.Fprintf(&b, "%s %s\n", view.Style(style.Warn, "warning:"), w.Error())
	}
	return b.String(), nil
}
