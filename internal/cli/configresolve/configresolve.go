// Package configresolve holds the logic of gero config resolve repos: it lists
// where each Project of the central config lives on this machine and how that
// was decided. It is framework-free: cmd/ owns the cobra wiring.
package configresolve

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrUnresolved is returned, with the table still written, when a Project has
// no local clone or several equally good ones.
var ErrUnresolved = errors.New("some repositories have no unique local clone")

// Workspace is what Repos needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Config() (config.Config, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// Repos resolves the local clone of each Project in the central config found
// from s and the working directory cwd, or of the one Project named by slugs,
// and renders the rows as s.Output (text or json). When any row is unresolved
// the output is still returned, with ErrUnresolved.
func Repos(s clicfg.Settings, cwd string, slugs []string) (string, error) {
	return repos(workspace.Open(s, cwd), s.Output, slugs)
}

func repos(ws Workspace, format string, slugs []string) (string, error) {
	return "", errs.NotImplemented("gero config resolve repos")
}
