package bump

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// StrategyRegistry maps the names strategies claim to their code.
type StrategyRegistry struct {
	strategies map[string]Strategy
}

// NewStrategyRegistry returns an empty registry.
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{strategies: map[string]Strategy{}}
}

// Builtin returns a registry with every strategy gero has, and the ones it names
// and has not built. The list is explicit on purpose, so what exists is visible
// here and not built up by init functions.
func Builtin() *StrategyRegistry {
	r := NewStrategyRegistry()
	r.Register(chartsPackageBump{})
	r.Register(goBump{})
	return r
}

// Register adds a strategy under the name it claims. Two strategies claiming one
// name is a programming error, so it panics.
func (r *StrategyRegistry) Register(s Strategy) {
	name := s.Name()
	if _, dup := r.strategies[name]; dup {
		panic(fmt.Sprintf("bump: two strategies claim the name %q", name))
	}
	r.strategies[name] = s
}

// Get returns the strategy of a name; the error lists what exists.
func (r *StrategyRegistry) Get(name string) (Strategy, error) {
	if s, ok := r.strategies[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("strategy %q is not known: use one of %s", name, strings.Join(r.Names(), ", "))
}

// Names lists the strategies, sorted.
func (r *StrategyRegistry) Names() []string {
	names := make([]string, 0, len(r.strategies))
	for n := range r.strategies {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// notBuilt is an Applier that says its strategy is declared and not built, so a
// run that needs it stops before it writes anything.
type notBuilt interface {
	NotBuilt() error
}

// chartsPackageBump is the rule that bumps a package of rancher/charts for a
// release of a chart. The real config names it, and it is not built: it is
// registered so that the config is valid, and a run that needs it says so.
type chartsPackageBump struct{}

// Name is how config refers to the strategy.
func (chartsPackageBump) Name() string { return "chartsPackageBump" }

// Configure reads `with`: the package, and nothing else.
func (s chartsPackageBump) Configure(with map[string]string) (Applier, error) {
	for key := range with {
		if key != "package" {
			return nil, fmt.Errorf("%s takes only with.package, not %q", s.Name(), key)
		}
	}
	if with["package"] == "" {
		return nil, fmt.Errorf("%s needs with.package, the package to bump", s.Name())
	}
	return unbuilt{name: s.Name()}, nil
}

type unbuilt struct{ name string }

func (u unbuilt) NotBuilt() error {
	return fmt.Errorf("%w: %s is named in the config and gero cannot run it yet", ErrNotBuilt, u.name)
}

func (u unbuilt) Apply(context.Context, WritableTree, StrategyInput) (Result, error) {
	return Result{}, u.NotBuilt()
}
