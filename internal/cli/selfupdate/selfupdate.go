// Package selfupdate holds the logic of gero doctor self-update: it looks for a
// newer release, says what it found, asks, and installs it. What proves the
// release is ours is internal/platform/selfupdate. It is framework-free: cmd/
// owns the cobra wiring.
package selfupdate

import (
	"context"
	"errors"
	"io"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	platform "github.com/rancher/orbs-automation/internal/platform/selfupdate"
)

// ErrNeedsApproval is returned when approval is needed and cannot be asked for:
// the input is not a terminal and --yes was not given.
var ErrNeedsApproval = errors.New("self-update replaces the gero binary and needs approval, but there is no terminal to ask on: pass --yes")

// ErrDeclined is returned when the user answers no. Nothing was changed.
var ErrDeclined = errors.New("not updated")

// Updater is what Run needs of internal/platform/selfupdate.
type Updater interface {
	Check(ctx context.Context) (platform.Status, error)
	Blocked() string
	Apply(ctx context.Context, s platform.Status) (string, error)
}

// Options are the choices of one run.
type Options struct {
	Updater Updater
	// CheckOnly says whether a newer release exists and stops. DryRun does the
	// same, naming what it would have installed.
	CheckOnly, DryRun bool
	// Yes approves without asking. Interactive says the input is a terminal.
	Yes, Interactive bool
	// In and Out are where the question is asked and answered.
	In  io.Reader
	Out io.Writer
	// Terminal is what the terminal can do, for the look of the report.
	Terminal clicfg.Terminal
}

// Run checks for a newer release and, when there is one and the user approves,
// installs it. It returns what to print.
func Run(ctx context.Context, o Options) (string, error) {
	return "", nil
}
