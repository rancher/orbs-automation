package doctor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/ghcli"
	"github.com/rancher/orbs-automation/internal/git/gitcli"
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

// GitBinary is what the git check needs from the git binary. It is defined
// here, where it is consumed, so tests can use a fake.
type GitBinary interface {
	// Version returns the first line of "git --version", or an error wrapping
	// gitcli.ErrNotFound when git is not installed.
	Version(ctx context.Context) (string, error)
	// Config returns what git would use for key, or "" when it is not set.
	Config(ctx context.Context, key string) (string, error)
}

// gitTimeout bounds the git calls of the git check, so a hung git cannot hang
// the run.
const gitTimeout = 5 * time.Second

// gitCheck looks for the git binary and the identity it would commit with. gero
// uses go-git for repositories and runs git only for a few operations, such as
// abbreviating a commit hash, so a missing git fails those commands and is a
// failure here; an unset identity only matters once gero makes commits, so it
// is a warning.
type gitCheck struct {
	git     GitBinary
	timeout time.Duration
}

// NewGitCheck returns the check named "git".
func NewGitCheck(g GitBinary) Check { return newGitCheck(g, gitTimeout) }

func newGitCheck(g GitBinary, timeout time.Duration) Check { return gitCheck{git: g, timeout: timeout} }

func (gitCheck) Name() string        { return "git" }
func (gitCheck) Category() string    { return "dependencies" }
func (gitCheck) Description() string { return "Looks for git and the name and email it commits with" }
func (gitCheck) Online() bool        { return false }
func (gitCheck) Needs() []string     { return nil }

func (gitCheck) Run(context.Context) Result { return Result{} }

// gitBinary is the GitBinary backed by internal/git/gitcli.
type gitBinary struct{}

func (gitBinary) Version(ctx context.Context) (string, error) { return gitcli.Version(ctx) }

func (gitBinary) Config(ctx context.Context, key string) (string, error) {
	return gitcli.Config(ctx, key)
}
