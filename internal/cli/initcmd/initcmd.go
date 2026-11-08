// Package initcmd holds the logic of gero init: it writes a new user file from
// a few answers, given as flags or asked on a terminal. It is framework-free:
// cmd/ owns the cobra wiring. The package is not named init, which Go reserves.
package initcmd

import (
	"errors"
	"io"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// ErrExists is returned when the user file exists and Force was not given.
var ErrExists = errors.New("the user file exists")

// ErrNeedsValues is returned when a value is missing and cannot be asked for:
// the input is not a terminal.
var ErrNeedsValues = errors.New("values are missing and there is no terminal to ask on")

// Options are the choices of one run.
type Options struct {
	// The four values of the file. One left empty is asked for on a terminal, and
	// is an error without one.
	GitHubUser, Fork, Upstream, BranchPrefix string
	// Force replaces a user file that exists, even one that cannot be loaded.
	Force bool
	// Interactive says the input is a terminal, where a missing value is asked.
	Interactive bool
	// DryRun shows the file and writes nothing.
	DryRun bool
	// In and Out are where a question is asked and answered.
	In  io.Reader
	Out io.Writer
	// Terminal is what the terminal can do, for the look of the report. The zero
	// value has no color.
	Terminal clicfg.Terminal
	// Getenv and Home find the user file, as clicfg.UserFilePath takes them.
	Getenv func(string) string
	Home   string
}

// Run writes the user file and returns the report to print.
func Run(Options) (string, error) {
	return "", nil
}
