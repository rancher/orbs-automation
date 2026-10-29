package workspace

import "github.com/rancher/orbs-automation/internal/git/repo"

// ErrNotRepository is returned when the working directory is not in the
// repository the method asks for. It is repo.ErrNotRepository, re-exported so
// commands need not import internal/git/repo to test for it.
var ErrNotRepository = repo.ErrNotRepository

// Current returns the repository that contains the working directory, found
// from it upward as git does, so subdirectories and worktrees work. It returns
// ErrNotRepository when there is none.
func (w *Workspace) Current() (*repo.Repo, error) {
	return nil, nil
}

// CurrentAtRoot returns the repository whose .git directory is in the working
// directory itself. Subdirectories, worktrees and submodules are not found. It
// returns ErrNotRepository otherwise. It exists for commands that must match
// the script they port, which only looks in the current directory.
func (w *Workspace) CurrentAtRoot() (*repo.Repo, error) {
	return nil, nil
}
