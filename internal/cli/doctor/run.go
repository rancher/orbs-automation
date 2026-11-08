package doctor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Flags are the command-line choices of one gero doctor run.
type Flags struct {
	Online     bool          // run the checks that need the network
	Strict     bool          // a warning fails the run
	ListChecks bool          // print the checks instead of running them
	Timeout    time.Duration // bound for each online check; zero means DefaultTimeout
	Names      []string      // run only these checks and what they need
}

// ErrChecksFailed is returned, after the report is printed, when the run should
// exit non-zero: a check failed, or one warned under --strict.
var ErrChecksFailed = errors.New("some doctor checks failed")

// Environment is what the checks that look at this machine need besides the
// build: the resolved settings, the working directory, and the home directory
// and environment the user file is found from.
type Environment struct {
	Settings clicfg.Settings
	Cwd      string
	Home     string
	Getenv   func(string) string
}

// Main runs gero doctor and returns the report in the format env.Settings.Output.
// The report is returned along with ErrChecksFailed, so the caller prints it and
// then exits non-zero.
func Main(ctx context.Context, env Environment, f Flags, b Build) (string, error) {
	return run(ctx, env.Settings, f, b, defaultChecks(b, env))
}

// forDoctor is the settings for gero doctor's own workspace. Doctor is offline
// unless asked to be online, so the central bundle is read from the cache only.
func forDoctor(s clicfg.Settings, online bool) clicfg.Settings {
	return s
}

// defaultChecks is every check gero doctor knows, in the order they run. A
// check's needs must come before it.
func defaultChecks(b Build, env Environment) []Check {
	ws := workspace.Open(env.Settings, env.Cwd)
	return []Check{
		NewVersionCheck(b),
		NewGitCheck(gitBinary{}),
		NewGhCheck(ghBinary{}),
		NewUserFileCheck(env),
		NewConfigDirCheck(ws),
		NewConfigCheck(ws),
		NewReposCheck(ws),
		NewAuthTokenCheck(github.GoGh{}),
		NewAuthAPICheck(authstatusVerifier{}),
		NewRateLimitCheck(githubRateLimiter{}, time.Now),
		NewCommitIdentityCheck(settingsResolver{env.Settings}, env.Settings.CI),
	}
}

func run(ctx context.Context, s clicfg.Settings, f Flags, b Build, checks []Check) (string, error) {
	// Refuse a format that cannot be rendered before running anything, since
	// an online check costs real requests.
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", unsupported(s.Output)
	}
	if f.ListChecks {
		return RenderChecks(checks, s.Output, style.New(s.Terminal))
	}
	outcomes, err := Run(ctx, checks, Options{Online: f.Online, Timeout: f.Timeout, Names: f.Names})
	if errors.Is(err, ErrUnknownCheck) {
		return "", fmt.Errorf("%w (see gero doctor --list-checks)", err)
	}
	if err != nil {
		return "", err
	}
	out, err := Render(Report{Build: b, Outcomes: outcomes, Style: style.New(s.Terminal)}, s.Output)
	if err != nil {
		return "", err
	}
	if Failed(outcomes, f.Strict) {
		return out, ErrChecksFailed
	}
	return out, nil
}
