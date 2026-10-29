package doctor

import (
	"context"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what the configuration checks need from the invocation's
// workspace, the one door to the central config and the local clones
// (docs/specs/WORKSPACE.md). It is defined here, where it is consumed, so tests
// can use a fake; *workspace.Workspace satisfies it.
type Workspace interface {
	ConfigDir() (workspace.ConfigDir, error)
	Config() (config.Config, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// configDirCheck finds the central config directory and says where it came from.
type configDirCheck struct{ ws Workspace }

// NewConfigDirCheck returns the check named "config-dir".
func NewConfigDirCheck(ws Workspace) Check { return configDirCheck{ws: ws} }

func (configDirCheck) Name() string     { return "config-dir" }
func (configDirCheck) Category() string { return "configuration" }
func (configDirCheck) Description() string {
	return "Finds the central config directory and says where it came from"
}
func (configDirCheck) Online() bool    { return false }
func (configDirCheck) Needs() []string { return nil }

func (configDirCheck) Run(context.Context) Result { return Result{} }

// configCheck loads the central application config and reports whether it is
// usable. It needs config-dir.
type configCheck struct{ ws Workspace }

// NewConfigCheck returns the check named "config".
func NewConfigCheck(ws Workspace) Check { return configCheck{ws: ws} }

func (configCheck) Name() string        { return "config" }
func (configCheck) Category() string    { return "configuration" }
func (configCheck) Description() string { return "Loads the central config and reports any problems" }
func (configCheck) Online() bool        { return false }
func (configCheck) Needs() []string     { return []string{"config-dir"} }

func (configCheck) Run(context.Context) Result { return Result{} }

// reposCheck looks for a local clone of each Project in the central config. It
// needs config.
type reposCheck struct{ ws Workspace }

// NewReposCheck returns the check named "repos".
func NewReposCheck(ws Workspace) Check { return reposCheck{ws: ws} }

func (reposCheck) Name() string     { return "repos" }
func (reposCheck) Category() string { return "configuration" }
func (reposCheck) Description() string {
	return "Looks for a local clone of each project in the central config"
}
func (reposCheck) Online() bool    { return false }
func (reposCheck) Needs() []string { return []string{"config"} }

func (reposCheck) Run(context.Context) Result { return Result{} }

// userFileCheck loads the user file the way every command does, so a file that
// would stop a command is reported here instead.
type userFileCheck struct{ env Environment }

// NewUserFileCheck returns the check named "user-file".
func NewUserFileCheck(env Environment) Check { return userFileCheck{env: env} }

func (userFileCheck) Name() string        { return "user-file" }
func (userFileCheck) Category() string    { return "configuration" }
func (userFileCheck) Description() string { return "Loads the user file the way every command does" }
func (userFileCheck) Online() bool        { return false }
func (userFileCheck) Needs() []string     { return nil }

func (userFileCheck) Run(context.Context) Result { return Result{} }
