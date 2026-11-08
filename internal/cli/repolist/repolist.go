// Package repolist holds the logic of gero repo list: which Projects of the
// central config have a clone on this machine, as three lists: cloned, not cloned
// and ambiguous. It reports what is on disk. gero config resolve repos reports how
// the config resolves each repository, and from which source. It is
// framework-free: cmd/ owns the cobra wiring.
package repolist

import (
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Clone is a Project that has a clone, and where it is.
type Clone struct {
	Slug string `json:"slug"`
	Path string `json:"path"`
}

// Ambiguous is a Project that has several equally good clones.
type Ambiguous struct {
	Slug       string   `json:"slug"`
	Candidates []string `json:"candidates"`
}

// Report is the three lists, each sorted by slug.
type Report struct {
	Cloned    []Clone     `json:"cloned"`
	NotCloned []string    `json:"notCloned"`
	Ambiguous []Ambiguous `json:"ambiguous"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Chain() (*chain.Graph, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// List finds which Projects of the central config found from s and the working
// directory cwd have a clone, and renders the lists as s.Output (text or json).
func List(s clicfg.Settings, cwd string) (string, error) {
	return list(workspace.Open(s, cwd), isDir, s.Output, style.New(s.Terminal))
}

func isDir(path string) bool { return false }

func list(ws Workspace, isDir func(string) bool, format string, view *style.Renderer) (string, error) {
	return "", nil
}
