// Package wanted says which release a Holding is expected to hold on a line: what config
// declares (chain) joined with what the host has (tags, and the go.mod of a pinning
// release). It decides nothing about a file's content, which bump checks against the
// release given here, and nothing about what to do with a difference, which is the
// caller's: ship, repo sync and repo status all ask here, so they cannot disagree.
package wanted

import (
	"context"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
)

// Source is what the host gives: the tags and branches of a repository, and a file of
// it at a ref.
type Source interface {
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// Want is what a Holding is expected to hold on a line, and why.
type Want struct {
	// Release is the release a release holding is expected to hold, and zero for a series.
	Release bump.Release
	// Series is the series a series holding is expected to hold.
	Series string
	// Why says where the value comes from: the newest release of the stream, the series
	// the stream has for the line, or the release a pin requires.
	Why string
	// RequiredBy is, for a release a pin requires, the pinning release whose file says so.
	RequiredBy string
	// Floor says the value is a floor: a newer one held holds it.
	Floor bool
	// Err is why there is no expected value: no release of the stream, or a pin that no release can be made of.
	Err error
}

// Resolver answers what a Holding is expected to hold, reading each repository's tags
// from the host once.
type Resolver struct {
	src Source
}

// New is a Resolver that reads the host through src.
func New(src Source) *Resolver { return &Resolver{src: src} }

// For is what h is expected to hold on a line: for a series, the series its stream has
// for the line; for a release, the newest stable release of its stream for the line (a
// holding that accepts all takes a prerelease too); for the virtual holding a pin makes,
// the release the go.mod of the pinning holding's release for the line requires.
func (r *Resolver) For(ctx context.Context, h *chain.Holding, line string) Want {
	return Want{}
}

// ForPinnerTag is what the virtual holding v is expected to hold when the holding that
// pins it holds the release pinnerTag: the version the go.mod of that release requires.
func (r *Resolver) ForPinnerTag(ctx context.Context, v *chain.Holding, pinnerTag string) Want {
	return Want{}
}
