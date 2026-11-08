package bump

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
)

// ErrNotBuilt is returned by a run that needs a strategy gero names and does not
// have yet.
var ErrNotBuilt = errors.New("strategy declared and not built")

// ErrNoBranches is returned when no consumer branch can be worked out for a
// release: the consumer has none to follow it to.
var ErrNoBranches = errors.New("no consumer branch for the release")

// StrategyInput is what a strategy may use to make its change: where the release
// goes, and which one it is.
type StrategyInput struct {
	Edit     string
	Producer *chain.Project
	Consumer *chain.Project
	Release  Release
	Line     string
	Branch   string
}

// Applier is a strategy configured for one edit: the extra change a named rule
// makes after the holdings are written.
type Applier interface {
	Apply(ctx context.Context, dst WritableTree, in StrategyInput) (Result, error)
}

// Strategy is code that claims a name, which an edit's `strategy` says. It is
// defined here and nowhere else.
type Strategy interface {
	Name() string
	// Configure checks the edit's `with` values and returns the Applier they
	// configure. It is where a typo in `with` is found, when the config loads.
	Configure(with map[string]string) (Applier, error)
}

// StrategyRegistry maps the names strategies claim to their code.
type StrategyRegistry struct{}

// NewStrategyRegistry returns an empty registry.
func NewStrategyRegistry() *StrategyRegistry { return &StrategyRegistry{} }

// Builtin returns a registry with every strategy gero has, and the ones it names
// and has not built.
func Builtin() *StrategyRegistry { return &StrategyRegistry{} }

// Register adds a strategy under the name it claims. Two strategies claiming one
// name is a programming error, so it panics.
func (r *StrategyRegistry) Register(Strategy) {}

// Get returns the strategy of a name; the error lists what exists.
func (r *StrategyRegistry) Get(string) (Strategy, error) { return nil, nil }

// Names lists the strategies, sorted.
func (r *StrategyRegistry) Names() []string { return nil }

// Engine knows the routes of the config and each edit's strategy, configured.
// Building it is where the mistakes that depend on a strategy or a format are
// found, which is also what gero doctor needs.
type Engine struct{}

// NewEngine builds the engine for a config: it configures the strategy of each
// edit from the registry and checks that every release holding is in a format
// gero can write. It reports every problem it finds, each naming the edit or the
// holding, as one joined error. The engine is returned either way, holding the
// routes that are fine.
func NewEngine(*StrategyRegistry, *chain.Graph) (*Engine, error) { return &Engine{}, nil }

// Route is one way out of a producer: the release holdings a consumer has of it.
type Route struct {
	Producer *chain.Project
	Consumer *chain.Project
	Holdings []*chain.Holding
}

// RoutesFrom returns the routes out of a producer, one for each consumer, in the
// order of the holdings, leaving out a route whose holdings or edits are not
// usable.
func (e *Engine) RoutesFrom(*chain.Project) []Route { return nil }

// RoutesInto returns the routes into a consumer, one for each producer, in the
// order of the holdings.
func (e *Engine) RoutesInto(*chain.Project) []Route { return nil }

// Edits returns the edits that follow the route's holdings, each once, in the
// order they are read.
func (r Route) Edits() []*chain.Edit { return nil }

// Target is a branch of the consumer a release goes to, and the holdings written
// there.
type Target struct {
	Branch   string
	Line     string // the Rancher line of the branch, empty for a branch of none
	Holdings []*chain.Holding
}

// TargetsFor returns the targets a release goes to, in order. line is the
// producer's line the release is in, or empty when it is in none.
func (r Route) TargetsFor(string) ([]Target, error) { return nil, nil }

// Phase is where in the lifecycle a step runs.
type Phase string

// The phases, in the order they run around the write.
const (
	PreEdit  Phase = "preEdit"
	PostEdit Phase = "postEdit"
)

// StepEnv is what a step runs with: the directory of the clone, and the
// environment the engine builds for it.
type StepEnv struct {
	Dir string
	Env map[string]string
}

// StepRunner runs one step. The engine decides which steps run and in what order;
// a runner owns how.
type StepRunner interface {
	Run(ctx context.Context, step v1alpha2.Step, env StepEnv) error
}

// StepRef is a step with the phase, the edit and the position it has.
type StepRef struct {
	Phase Phase
	Edit  string
	Index int
	Step  v1alpha2.Step
}

// Steps returns the steps that run for a target, in the order they run: the
// edits' pre-edit steps before the write and their post-edit steps after it.
func (r Route) Steps(Target) (pre, post []StepRef) { return nil, nil }

// StepError is a step that failed, naming where it was.
type StepError struct {
	Phase Phase
	Edit  string
	Index int
	Run   []string
	Err   error
}

func (e *StepError) Error() string { return "" }

func (e *StepError) Unwrap() error { return e.Err }

// UnitKind says what a Unit is.
type UnitKind string

// The kinds of unit.
const (
	UnitStep     UnitKind = "step"
	UnitEdit     UnitKind = "edit" // the holdings written
	UnitStrategy UnitKind = "strategy"
)

// Unit is one piece of the lifecycle that may leave changes behind.
type Unit struct {
	Kind    UnitKind
	Phase   Phase    // for a step: where it runs
	Edit    string   // for a step or a strategy: the edit it is of
	Index   int      // for a step: its position in that edit's list
	Run     []string // for a step: its command
	Changes []Change // for the write and a strategy: what it changed
}

// Checkpoint is called after each unit has finished, so the caller can act on
// what it left behind, such as committing it. An error stops the run.
type Checkpoint func(ctx context.Context, u Unit) error

// Ready says whether the route can be run: nothing it needs is declared and not
// built.
func (r Route) Ready() error { return nil }

// HoldingCheck is what one holding of a target holds.
type HoldingCheck struct {
	Holding *chain.Holding
	Check
}

// Check reads what each holding of the target holds in src and how it compares
// with the release of tag. It never writes.
func (r Route) Check(context.Context, Tree, Target, string, string) ([]HoldingCheck, error) {
	return nil, nil
}

// Run runs the lifecycle of the target in dir: the pre-edit steps, the write of
// each holding, the strategies, then the post-edit steps, calling after (when it
// is not nil) once each has finished. A step that fails stops the run, as a
// *StepError.
func (r Route) Run(context.Context, WritableTree, StepRunner, Target, string, string, string, Checkpoint) (Result, error) {
	return Result{}, nil
}
