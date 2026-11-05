// Package workspace is the one door from CLI Settings to the world a command
// acts on: the central config directory today, later the loaded application
// config and the local path, upstream and fork of a repo
// (docs/specs/WORKSPACE.md and docs/specs/CLI_CONFIG.md). Each rule lives here once, so
// the same inputs resolve the same way in every command. It is framework-free
// and does no I/O until a method needs it.
package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/repo"
	"github.com/rancher/orbs-automation/internal/github"
)

// NotFoundHint is the advice for a missing central config directory. It is the
// tail of ErrNotFound, and gero doctor shows it as the fix.
const NotFoundHint = "set one of --config-dir, GERO_CONFIG_DIR or configDir in the user file, " +
	"or run from a checkout of the central repo (./config)"

// ErrNotFound is returned when no source names a config directory.
var ErrNotFound = errors.New("central config directory not found: " + NotFoundHint)

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

	// The network reads, injected so tests need no network or credentials.
	remoteTags  func(ctx context.Context, url, token string) ([]git.Tag, error)
	credentials func() (host, token string)
}

// Open returns the Workspace for s and the working directory cwd, reading the
// real file system there.
func Open(s clicfg.Settings, cwd string) *Workspace {
	return open(s, cwd, os.DirFS(cwd))
}

// open is Open with the working directory's file system injected.
func open(s clicfg.Settings, cwd string, cwdFS fs.FS) *Workspace {
	return &Workspace{
		settings: s, cwd: cwd, cwdFS: cwdFS,
		remoteTags:  repo.ListRemoteTags,
		credentials: defaultCredentials,
	}
}

// defaultCredentials is the host gero talks to and the token for it, empty when
// none is found. A missing token is not an error here: reading a public
// repository needs none.
func defaultCredentials() (host, token string) {
	auth := github.GoGh{}
	host, _ = auth.DefaultHost()
	if c, err := github.Resolve(auth, host); err == nil {
		token = c.Token
	}
	return host, token
}

// ConfigDir returns the central config directory: the configDir setting when
// set (flag, env or user file), else ./config when the working directory holds
// config/*.project.yaml. It returns ErrNotFound when neither names one.
func (w *Workspace) ConfigDir() (ConfigDir, error) {
	if w.settings.ConfigDir != "" {
		return ConfigDir{Dir: w.settings.ConfigDir, Source: w.settings.Source("configDir").String()}, nil
	}
	if w.cwdFS != nil {
		if m, err := fs.Glob(w.cwdFS, "config/*.project.yaml"); err == nil && len(m) > 0 {
			return ConfigDir{Dir: filepath.Join(w.cwd, "config"), Source: SourceCwd}, nil
		}
	}
	return ConfigDir{}, ErrNotFound
}
