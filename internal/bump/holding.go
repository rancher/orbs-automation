package bump

import (
	"context"
	"errors"

	"github.com/rancher/orbs-automation/internal/chain"
)

// ErrFormatNotBuilt is returned for a holding whose format gero cannot read or write
// yet.
var ErrFormatNotBuilt = errors.New("format not built")

// ReleaseOf makes the variables of a release of the producer for what a holding
// holds of it: the tag, the artifact's image or file, and where the file is among
// the release's files.
func ReleaseOf(_ *chain.Holding, _, _ string) (Release, error) { return Release{}, nil }

// Place is where a holding is found in a consumer's repo for one target.
type Place struct {
	File   string
	Key    string
	Format string
}

// PlaceOf renders where a holding is found for a line and a branch of the
// consumer: its file and key with the line, the holding's stage and the branch.
func PlaceOf(_ *chain.Holding, _, _ string) (Place, error) { return Place{}, nil }

// FormOf renders how a holding writes the release: its form, with the release's
// variables.
func FormOf(_ *chain.Holding, _ Release) (string, error) { return "", nil }

// CheckHolding reads what a holding holds in src for a line and a branch, and
// says how it compares with what the release wants. It never writes.
func CheckHolding(_ context.Context, _ Tree, _ *chain.Holding, _, _ string, _ Release) (Check, error) {
	return Check{}, nil
}

// ApplyHolding writes the release into the holding's place in dst, changing only
// that value, and says what changed.
func ApplyHolding(_ context.Context, _ WritableTree, _ *chain.Holding, _, _ string, _ Release) (Result, error) {
	return Result{}, nil
}
