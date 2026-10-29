// Package doctor holds the logic of gero doctor: the checks that say whether a
// workstation is set up to use gero, the runner that orders and gates them, and
// the renderers (docs/specs/DOCTOR.md). It is framework-free: cmd/ owns the
// cobra wiring.
package doctor

import (
	"context"
	"errors"
	"time"
)

// Status is the outcome of one check.
type Status int

// The statuses, from best to worst, with skip last.
const (
	StatusOK   Status = iota // works as expected
	StatusWarn               // works, but something may bite later
	StatusFail               // gero cannot do its job until this is fixed
	StatusSkip               // not run: needs --online, or a prerequisite did not pass
)

// String is the name used in text and JSON output.
func (Status) String() string { return "" }

// Result is what a check reports.
type Result struct {
	Status      Status
	Message     string // one line
	Remediation string // what to do, for a warn or a fail
}

// Check is one question with one answer. It is defined here, where it is
// consumed, so tests can use a fake.
type Check interface {
	// Name is a short stable kebab-case name, used on the command line and in JSON.
	Name() string
	// Category groups the check in the output.
	Category() string
	// Description is one line for --list-checks.
	Description() string
	// Online reports whether the check needs the network.
	Online() bool
	// Needs names the checks that must be ok or warn first. Each must be
	// registered before this one.
	Needs() []string
	// Run answers the question. It should honor ctx.
	Run(ctx context.Context) Result
}

// Outcome is a check's result as the runner reports it.
type Outcome struct {
	Name        string
	Category    string
	Online      bool
	Status      Status
	Message     string
	Remediation string
	SkipReason  string // set when Status is StatusSkip
}

// Options controls one run.
type Options struct {
	// Online lets the checks that need the network run.
	Online bool
	// Timeout bounds each online check. Zero means DefaultTimeout.
	Timeout time.Duration
	// Names selects checks by name; empty runs all of them. The checks they
	// need run too. Naming an online check allows online checks.
	Names []string
}

// DefaultTimeout bounds an online check when Options.Timeout is zero.
const DefaultTimeout = 10 * time.Second

var (
	// ErrUnknownCheck is returned when a requested name matches no check.
	ErrUnknownCheck = errors.New("unknown check")
	// ErrBadRegistry is returned when the checks themselves are inconsistent:
	// a duplicate name, or a need that is unknown or not registered earlier.
	ErrBadRegistry = errors.New("invalid check registry")
)

// Run runs checks in the order given and returns one Outcome per check that was
// selected.
func Run(_ context.Context, _ []Check, _ Options) ([]Outcome, error) {
	return nil, nil
}

// Counts tallies outcomes by status.
type Counts struct{ OK, Warn, Fail, Skip int }

// Summarize counts the outcomes by status.
func Summarize(_ []Outcome) Counts { return Counts{} }

// NeedOnline returns how many outcomes were skipped only because --online was
// not given.
func NeedOnline(_ []Outcome) int { return 0 }

// Failed reports whether the run should exit non-zero: any fail, or a warn when
// strict.
func Failed(_ []Outcome, _ bool) bool { return false }
