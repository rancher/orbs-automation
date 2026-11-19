// Package wanted says which release a Holding is expected to hold on a line: what config
// declares (chain) joined with what the host has (tags, and the go.mod of a pinning
// release). It decides nothing about a file's content, which bump checks against the
// release given here, and nothing about what to do with a difference, which is the
// caller's: ship, repo sync and repo status all ask here, so they cannot disagree.
package wanted

import (
	"context"
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/tagscheme"
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
	src  Source
	tags map[string]tagsResult
}

type tagsResult struct {
	tags []git.Tag
	err  error
}

// New is a Resolver that reads the host through src.
func New(src Source) *Resolver { return &Resolver{src: src, tags: map[string]tagsResult{}} }

// tagsOf is the tags of a repository, read from the host the first time.
func (r *Resolver) tagsOf(ctx context.Context, slug string) ([]git.Tag, error) {
	if got, ok := r.tags[slug]; ok {
		return got.tags, got.err
	}
	refs, err := r.src.UpstreamRefs(ctx, slug)
	r.tags[slug] = tagsResult{refs.Tags, err}
	return refs.Tags, err
}

// cached is the Source of a Resolver for what reads only tags: the host asked once.
type cached struct{ r *Resolver }

func (c cached) UpstreamRefs(ctx context.Context, slug string) (git.Refs, error) {
	tags, err := c.r.tagsOf(ctx, slug)
	return git.Refs{Tags: tags}, err
}

func (c cached) UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error) {
	return c.r.src.UpstreamFile(ctx, slug, ref, path)
}

// For is what h is expected to hold on a line: for a series, the series its stream has
// for the line; for a release, the newest stable release of its stream for the line (a
// holding that accepts all takes a prerelease too); for the virtual holding a pin makes,
// the release the go.mod of the pinning holding's release for the line requires.
func (r *Resolver) For(ctx context.Context, h *chain.Holding, line string) Want {
	switch {
	case h.Kind() == chain.HoldsSeries:
		return seriesFor(h, line)
	case h.Virtual():
		pinner := r.For(ctx, h.PinnedBy(), line)
		if pinner.Err != nil {
			return Want{Err: fmt.Errorf("%s, which pins it: %w", h.PinnedBy().Name(), pinner.Err)}
		}
		return r.ForPinnerTag(ctx, h, pinner.Release.Tag)
	}
	return r.newest(ctx, h, line)
}

// seriesFor is the series the stream of a series holding has for the line.
func seriesFor(h *chain.Holding, line string) Want {
	stream := h.Stream()
	series, ok := stream.Series(line)
	if !ok {
		return Want{Err: fmt.Errorf("Stream %q has no series for %s, so Holding %q has nothing to hold there", stream.Name(), line, h.Name())}
	}
	return Want{Series: series, Why: fmt.Sprintf("the series `%s` of `%s` for %s", series, stream.Name(), line)}
}

// newest is the newest stable release of the stream of a release holding for a line: of
// the series the stream has for the line, or of every tag it accepts when the stream is
// universal. A holding that accepts all takes a prerelease too.
func (r *Resolver) newest(ctx context.Context, h *chain.Holding, line string) Want {
	stream := h.Artifact().Stream()
	slug := h.Producer().Slug()
	scheme, err := tagscheme.ForStream(stream)
	if err != nil {
		return Want{Err: err}
	}
	all, err := r.tagsOf(ctx, slug)
	if err != nil {
		return Want{Err: fmt.Errorf("read the tags of %s: %w", slug, err)}
	}
	arg := ""
	if !stream.Universal() {
		arg = line
	}
	rel, err := tagscheme.ResolveRelease(scheme, slug, all, arg, h.AcceptsAll(), h.AcceptsAll())
	if errors.Is(err, tagscheme.ErrNoRelease) {
		return Want{Err: fmt.Errorf("%s has no stable release of %s to hold for %s", slug, stream.Name(), line)}
	}
	if err != nil {
		return Want{Err: err}
	}
	release, err := bump.ReleaseOf(h, rel.Tag, rel.Commit)
	if err != nil {
		return Want{Err: err}
	}
	return Want{Release: release, Why: fmt.Sprintf("the newest release of `%s`", stream.Name())}
}

// ForPinnerTag is what the virtual holding v is expected to hold when the holding that
// pins it holds the release pinnerTag: the version the go.mod of that release requires.
func (r *Resolver) ForPinnerTag(ctx context.Context, v *chain.Holding, pinnerTag string) Want {
	c := Derive(ctx, cached{r}, v, pinnerTag)
	if c.Error != "" {
		return Want{Err: errors.New(c.Error), RequiredBy: c.RequiredBy}
	}
	release, err := bump.ReleaseOf(v, c.Tag, "")
	if err != nil {
		return Want{Err: err, RequiredBy: c.RequiredBy}
	}
	return Want{
		Release: release, RequiredBy: c.RequiredBy, Floor: true,
		Why: fmt.Sprintf("`%s`, which the pin of `%s` requires", c.Tag, c.RequiredBy),
	}
}
