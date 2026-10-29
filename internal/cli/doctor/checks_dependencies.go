package doctor

import (
	"context"
	"time"
)

// GhBinary is what the gh check needs from the gh binary. It is defined here,
// where it is consumed, so tests can use a fake.
type GhBinary interface {
	// Locate returns the path of the gh that token resolution would run.
	Locate() (string, error)
	// Version returns the first line of "gh --version" for the binary at path.
	Version(ctx context.Context, path string) (string, error)
}

// ghVersionTimeout bounds "gh --version", so a hung gh cannot hang the run.
const ghVersionTimeout = 5 * time.Second

// ghCheck looks for the gh binary. gero reads a token from the gh keyring
// through it, so a missing gh is a warning, never a failure: GH_TOKEN works
// without it.
type ghCheck struct {
	gh      GhBinary
	timeout time.Duration
}

// NewGhCheck returns the check named "gh".
func NewGhCheck(g GhBinary) Check { return newGhCheck(g, ghVersionTimeout) }

func newGhCheck(g GhBinary, timeout time.Duration) Check { return ghCheck{gh: g, timeout: timeout} }

func (ghCheck) Name() string     { return "gh" }
func (ghCheck) Category() string { return "dependencies" }
func (ghCheck) Description() string {
	return "Looks for the gh binary that supplies a token from its keyring"
}
func (ghCheck) Online() bool    { return false }
func (ghCheck) Needs() []string { return nil }

func (ghCheck) Run(context.Context) Result { return Result{} }
