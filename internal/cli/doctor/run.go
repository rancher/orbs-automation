package doctor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
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

// Main runs gero doctor and returns the report in the format s.Output. The
// report is returned along with ErrChecksFailed, so the caller prints it and
// then exits non-zero.
func Main(ctx context.Context, s clicfg.Settings, f Flags, b Build) (string, error) {
	return run(ctx, s, f, b, defaultChecks(b))
}

// defaultChecks is every check gero doctor knows, in the order they run. A
// check's needs must come before it.
func defaultChecks(b Build) []Check {
	return []Check{
		NewVersionCheck(b),
		NewGhCheck(ghBinary{}),
	}
}

func run(ctx context.Context, s clicfg.Settings, f Flags, b Build, checks []Check) (string, error) {
	// Refuse a format that cannot be rendered before running anything, since
	// an online check costs real requests.
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", unsupported(s.Output)
	}
	if f.ListChecks {
		return RenderChecks(checks, s.Output)
	}
	outcomes, err := Run(ctx, checks, Options{Online: f.Online, Timeout: f.Timeout, Names: f.Names})
	if errors.Is(err, ErrUnknownCheck) {
		return "", fmt.Errorf("%w (see gero doctor --list-checks)", err)
	}
	if err != nil {
		return "", err
	}
	out, err := Render(Report{Build: b, Outcomes: outcomes}, s.Output)
	if err != nil {
		return "", err
	}
	if Failed(outcomes, f.Strict) {
		return out, ErrChecksFailed
	}
	return out, nil
}
