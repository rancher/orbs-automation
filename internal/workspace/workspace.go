// Package workspace is the one door from CLI Settings to the world a command
// acts on: the central config directory today, later the loaded application
// config and the local path, upstream and fork of a repo
// (docs/specs/CLI_CONFIG.md sections 3a and 4). Each rule lives here once, so
// the same inputs resolve the same way in every command. It is framework-free
// and does no I/O until a method needs it.
package workspace

import (
	"errors"
	"io/fs"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

// ErrNotFound is returned when no source names a config directory.
var ErrNotFound = errors.New("central config directory not found: set one of --config-dir, " +
	"GERO_CONFIG_DIR or configDir in the user file, or run from a checkout of the central repo (./config)")

// SourceCwd is the ConfigDir.Source of a directory found in the working
// directory.
const SourceCwd = "cwd"

// ConfigDir is the resolved central config directory and where it came from:
// the clicfg layer (flag, env or file) or SourceCwd.
type ConfigDir struct {
	Dir    string
	Source string
}

// Workspace resolves what one invocation acts on from its Settings and working
// directory.
type Workspace struct {
	settings clicfg.Settings
	cwd      string
	cwdFS    fs.FS
}

// Open returns the Workspace for s and the working directory cwd, reading the
// real file system there.
func Open(s clicfg.Settings, cwd string) *Workspace {
	return nil
}

// open is Open with the working directory's file system injected.
func open(s clicfg.Settings, cwd string, cwdFS fs.FS) *Workspace {
	return &Workspace{settings: s, cwd: cwd, cwdFS: cwdFS}
}

// ConfigDir returns the central config directory: the configDir setting when
// set (flag, env or user file), else ./config when the working directory holds
// config/*.project.yaml. It returns ErrNotFound when neither names one.
func (w *Workspace) ConfigDir() (ConfigDir, error) {
	return ConfigDir{}, nil
}
