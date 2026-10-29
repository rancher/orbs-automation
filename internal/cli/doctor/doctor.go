// Package doctor holds the logic of gero doctor: the checks that say whether a
// workstation is set up to use gero, the runner that orders and gates them, and
// the renderers (docs/specs/DOCTOR.md). It is framework-free: cmd/ owns the
// cobra wiring.
package doctor

import (
	"context"
	"errors"
	"fmt"
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
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	case StatusSkip:
		return "skip"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

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

// skipOnline is the SkipReason of an online check that was not asked for.
const skipOnline = "run with --online"

// Run runs checks in the order given and returns one Outcome per check that was
// selected. Checks are independent: one that fails, times out or panics never
// stops the others. It returns an error only for a bad request or registry, or
// when ctx is cancelled, with the outcomes so far.
func Run(ctx context.Context, checks []Check, opts Options) ([]Outcome, error) {
	index, err := validate(checks)
	if err != nil {
		return nil, err
	}
	selected, namedOnline, err := selection(checks, index, opts.Names)
	if err != nil {
		return nil, err
	}
	allowOnline := opts.Online || namedOnline
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	var outcomes []Outcome
	status := make(map[string]Status, len(checks))
	for i, c := range checks {
		if !selected[i] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return outcomes, err
		}
		o := Outcome{Name: c.Name(), Category: c.Category(), Online: c.Online()}
		switch need := blockedBy(c, status); {
		case c.Online() && !allowOnline:
			o.Status, o.SkipReason = StatusSkip, skipOnline
		case need != "":
			o.Status, o.SkipReason = StatusSkip, "needs "+need
		default:
			r, err := runCheck(ctx, c, timeout)
			if err != nil {
				return outcomes, err
			}
			o.Status, o.Message, o.Remediation = r.Status, r.Message, r.Remediation
		}
		status[o.Name] = o.Status
		outcomes = append(outcomes, o)
	}
	return outcomes, nil
}

// validate maps each check name to its position and rejects a registry the
// runner cannot order: a duplicate name, or a need that is unknown or not
// registered before the check that has it.
func validate(checks []Check) (map[string]int, error) {
	index := make(map[string]int, len(checks))
	for i, c := range checks {
		if _, dup := index[c.Name()]; dup {
			return nil, fmt.Errorf("%w: duplicate check %q", ErrBadRegistry, c.Name())
		}
		for _, need := range c.Needs() {
			if j, ok := index[need]; !ok || j >= i {
				return nil, fmt.Errorf("%w: %q needs %q, which is not registered before it", ErrBadRegistry, c.Name(), need)
			}
		}
		index[c.Name()] = i
	}
	return index, nil
}

// selection marks the checks to run: all of them when names is empty, else the
// named ones and everything they need. namedOnline reports whether a named
// check is online, which is the opt-in to run online checks.
func selection(checks []Check, index map[string]int, names []string) (selected []bool, namedOnline bool, err error) {
	selected = make([]bool, len(checks))
	if len(names) == 0 {
		for i := range selected {
			selected[i] = true
		}
		return selected, false, nil
	}
	var mark func(i int)
	mark = func(i int) {
		if selected[i] {
			return
		}
		selected[i] = true
		for _, need := range checks[i].Needs() {
			mark(index[need])
		}
	}
	for _, n := range names {
		i, ok := index[n]
		if !ok {
			return nil, false, fmt.Errorf("%w %q", ErrUnknownCheck, n)
		}
		namedOnline = namedOnline || checks[i].Online()
		mark(i)
	}
	return selected, namedOnline, nil
}

// blockedBy returns the first need of c that failed or was skipped, or "".
func blockedBy(c Check, status map[string]Status) string {
	for _, need := range c.Needs() {
		if s := status[need]; s == StatusFail || s == StatusSkip {
			return need
		}
	}
	return ""
}

// runCheck runs c and turns a timeout into a warning and a panic into a
// failure. It does not wait for a check that ignores its context. It returns an
// error only when the caller cancelled ctx.
func runCheck(ctx context.Context, c Check, timeout time.Duration) (Result, error) {
	runCtx := ctx
	if c.Online() {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	done := make(chan Result, 1) // buffered, so an abandoned check can still finish
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- Result{
					Status:      StatusFail,
					Message:     fmt.Sprintf("check panicked: %v", r),
					Remediation: "this is a bug in gero: please report it",
				}
			}
		}()
		done <- c.Run(runCtx)
	}()

	var r Result
	select {
	case r = <-done:
	case <-runCtx.Done():
		r = Result{Status: StatusFail}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) && r.Status != StatusOK {
		// A flaky network is not a broken setup.
		return Result{
			Status:      StatusWarn,
			Message:     fmt.Sprintf("timed out after %s", timeout),
			Remediation: "check your network connection and run again, or raise --timeout",
		}, nil
	}
	return r, nil
}

// Counts tallies outcomes by status.
type Counts struct{ OK, Warn, Fail, Skip int }

// Summarize counts the outcomes by status.
func Summarize(outcomes []Outcome) Counts {
	var n Counts
	for _, o := range outcomes {
		switch o.Status {
		case StatusOK:
			n.OK++
		case StatusWarn:
			n.Warn++
		case StatusFail:
			n.Fail++
		case StatusSkip:
			n.Skip++
		}
	}
	return n
}

// NeedOnline returns how many outcomes were skipped only because --online was
// not given.
func NeedOnline(outcomes []Outcome) int {
	n := 0
	for _, o := range outcomes {
		if o.Status == StatusSkip && o.SkipReason == skipOnline {
			n++
		}
	}
	return n
}

// Failed reports whether the run should exit non-zero: any fail, or a warn when
// strict.
func Failed(outcomes []Outcome, strict bool) bool {
	n := Summarize(outcomes)
	return n.Fail > 0 || (strict && n.Warn > 0)
}
