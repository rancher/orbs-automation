// Package configdir finds the central application config directory for one
// invocation (docs/specs/CLI_CONFIG.md section 4). It is framework-free and
// touches the file system only through the fs.FS it is given.
package configdir

import (
	"errors"
	"io/fs"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

// ErrNotFound is returned when no source names a config directory.
var ErrNotFound = errors.New("central config directory not found")

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
func Resolve(clicfg.Settings, string, fs.FS) (Result, error) {
	return Result{}, nil
}
