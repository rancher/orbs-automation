package doctor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/ghcli"
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

func (c ghCheck) Run(ctx context.Context) Result {
	path, err := c.gh.Locate()
	if err != nil {
		return Result{
			Status:  StatusWarn,
			Message: err.Error(),
			Remediation: "gero reads a token from the gh keyring through it: install it from https://cli.github.com, " +
				"or set GH_TOKEN (or fix GH_PATH, if you set it)",
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	version, err := c.gh.Version(ctx, path)
	if err != nil {
		return Result{
			Status:      StatusWarn,
			Message:     fmt.Sprintf("found at %s, but running it failed: %v", path, err),
			Remediation: "run `gh --version` yourself to see why",
		}
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("%s at %s", version, path)}
}

// ghBinary is the GhBinary backed by internal/ghcli.
type ghBinary struct{}

func (ghBinary) Locate() (string, error) { return ghcli.Locate(os.Getenv) }

func (ghBinary) Version(ctx context.Context, path string) (string, error) {
	return ghcli.Version(ctx, path)
}
