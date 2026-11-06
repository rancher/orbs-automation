package updatedispatcher

import (
	"fmt"
	"slices"
	"strings"
)

// StrategyRegistry maps the names strategies claim to their code.
type StrategyRegistry struct {
	strategies map[string]UpdateStrategy
}

// NewStrategyRegistry returns an empty registry.
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{strategies: map[string]UpdateStrategy{}}
}

// Builtin returns a registry with every strategy gero has. The list is explicit
// on purpose, so what exists is visible here and not built up by init functions.
func Builtin() *StrategyRegistry {
	r := NewStrategyRegistry()
	r.Register(YAMLBump{})
	return r
}

// Register adds a strategy under the name it claims. Two strategies claiming one
// name is a programming error, so it panics.
func (r *StrategyRegistry) Register(s UpdateStrategy) {
	name := s.Name()
	if _, dup := r.strategies[name]; dup {
		panic(fmt.Sprintf("updatedispatcher: two strategies claim the name %q", name))
	}
	r.strategies[name] = s
}

// Get returns the strategy of a name; the error lists what exists.
func (r *StrategyRegistry) Get(name string) (UpdateStrategy, error) {
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
