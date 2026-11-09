package bump

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"

	"github.com/rancher/orbs-automation/internal/chain"
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

// tidy is the program goBump runs in each module.
var tidy = []string{"go", "mod", "tidy"}

// Apply runs `go mod tidy` in the directory of each go.mod the holdings of the
// edit wrote, once for each, the root module first when it is among them. It
// reports a change for a module whose go.mod or go.sum tidy changed, so the pull
// request names only what happened. A module that tidy cannot tidy stops it,
// naming the directory.
func (goBumpApplier) Apply(ctx context.Context, dst WritableTree, in StrategyInput) (Result, error) {
	var dirs []string // each module's directory, "." for the root
	for _, c := range in.Changes {
		if path.Base(c.Path) != "go.mod" {
			continue
		}
		if dir := path.Dir(c.Path); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) > 0 && in.Runner == nil {
		return Result{}, errors.New("goBump has no way to run go mod tidy")
	}
	var res Result
	for _, dir := range dirs {
		before := moduleFiles(ctx, dst, dir)
		if err := in.Runner.Run(ctx, chain.Step{Run: tidy}, StepEnv{Dir: filepath.Join(in.Dir, filepath.FromSlash(dir))}); err != nil {
			return res, fmt.Errorf("go mod tidy in %s: %w", dir, err)
		}
		if moduleFiles(ctx, dst, dir) == before {
			continue
		}
		res.Changed = true
		res.Changes = append(res.Changes, Change{Path: path.Join(dir, "go.mod"), Description: tidyDescription(dir)})
	}
	return res, nil
}

// moduleFilesText is the text of a module's go.mod and go.sum, with a file that is
// not there as empty.
type moduleFilesText struct{ mod, sum string }

// moduleFiles reads them from the tree.

func moduleFiles(ctx context.Context, dst WritableTree, dir string) moduleFilesText {
	read := func(name string) string {
		data, err := dst.ReadFile(ctx, path.Join(dir, name))
		if err != nil {
			return ""
		}
		return string(data)
	}
	return moduleFilesText{mod: read("go.mod"), sum: read("go.sum")}
}

func tidyDescription(dir string) string {
	if dir == "." {
		return "Ran `go mod tidy`"
	}
	return fmt.Sprintf("Ran `go mod tidy` in `%s`", dir)
}
