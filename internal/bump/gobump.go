package bump

import (
	"context"
	"fmt"
)

// goBump is the rule for a Go module that holds another module's release: after
// the holdings write the new version into each go.mod, it runs `go mod tidy` in
// each of those modules, so go.sum and the rest of the file agree with it. It
// takes no `with` values. Each of those is its own commit: the engine commits the
// holdings' write and then this, so a reviewer sees the require line change and
// then what tidy did.
type goBump struct{}

// Name is how config refers to the strategy.
func (goBump) Name() string { return "goBump" }

// Configure takes nothing: a value in `with` is a typo.
func (s goBump) Configure(with map[string]string) (Applier, error) {
	if len(with) > 0 {
		return nil, fmt.Errorf("%s takes no with values", s.Name())
	}
	return goBumpApplier{}, nil
}

type goBumpApplier struct{}

func (goBumpApplier) Apply(context.Context, WritableTree, StrategyInput) (Result, error) {
	return Result{}, nil
}
