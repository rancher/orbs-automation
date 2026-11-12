// Package cdpath holds the logic of gero cd: it finds the local clone of a
// Project by the user's own config and returns its path, for a shell function
// to cd into. A program cannot change its parent shell's directory, so the
// command only prints. It is framework-free: cmd/ owns the cobra wiring.
package cdpath

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrProjectRequired is returned when no project is named, or "." is.
var ErrProjectRequired = errors.New("name a project by its config name or owner/name slug: the working directory is already where . would go")

// Workspace is what Path needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug.
	Project(key string) (*chain.Project, error)
	// ConfiguredRepoPath finds the clone by repos.paths and repos.search only.
	ConfiguredRepoPath(slug string) (workspace.RepoPath, error)
}

// Path returns the directory of the clone of the Project named by args, and a
// newline, found from s and the working directory cwd. There must be exactly
// one argument, and it is never the implicit project: the working directory
// plays no part.
func Path(s clicfg.Settings, cwd string, args []string) (string, error) {
	return path(workspace.Open(s, cwd), args, isDir)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func path(ws Workspace, args []string, isDir func(string) bool) (string, error) {
	if len(args) != 1 || args[0] == "" || args[0] == "." {
		return "", ErrProjectRequired
	}
	p, err := ws.Project(args[0])
	if err != nil {
		return "", err
	}
	rp, err := ws.ConfiguredRepoPath(p.Slug())
	switch {
	case errors.Is(err, workspace.ErrRepoAmbiguous):
		return "", fmt.Errorf("%s: %w: %s; set repos.paths.%s to choose one",
			p.Slug(), err, strings.Join(rp.Candidates, ", "), p.Slug())
	case err != nil:
		return "", fmt.Errorf("%s: %w: set repos.paths.%s, or clone it under a repos.search directory",
			p.Slug(), err, p.Slug())
	}
	if !isDir(rp.Path) {
		return "", fmt.Errorf("%s: %s (from %s) is not a directory", p.Slug(), rp.Path, rp.Source)
	}
	return rp.Path + "\n", nil
}
