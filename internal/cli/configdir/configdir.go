// Package configdir finds the central application config directory for one
// invocation (docs/specs/CLI_CONFIG.md section 4). It is framework-free and
// touches the file system only through the fs.FS it is given.
package configdir

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

// ErrNotFound is returned when no source names a config directory.
var ErrNotFound = errors.New("central config directory not found: set one of --config-dir, " +
	"GERO_CONFIG_DIR or configDir in the user file, or run from a checkout of the central repo (./config)")

// SourceCwd is the Result.Source of a directory found in the working directory.
const SourceCwd = "cwd"

// Result is the resolved directory and where it came from: the clicfg layer
// (flag, env or file) or SourceCwd.
type Result struct {
	Dir    string
	Source string
}

// Resolve returns the central config directory: the configDir setting when set
// (flag, env or user file), else ./config when cwdFS, the file system rooted at
// the working directory cwd, holds config/*.project.yaml.
func Resolve(s clicfg.Settings, cwd string, cwdFS fs.FS) (Result, error) {
	if s.ConfigDir != "" {
		return Result{Dir: s.ConfigDir, Source: s.Source("configDir").String()}, nil
	}
	if cwdFS != nil {
		if m, err := fs.Glob(cwdFS, "config/*.project.yaml"); err == nil && len(m) > 0 {
			return Result{Dir: filepath.Join(cwd, "config"), Source: SourceCwd}, nil
		}
	}
	return Result{}, ErrNotFound
}
