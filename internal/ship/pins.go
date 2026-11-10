package ship

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/gomodedit"
)

// Companion is a release shipped with the one asked for, because the Artifact of
// that release pins the Artifact of this one (wrangler's go.mod requires a lasso):
// the version it requires, where it was read, and whether it can be shipped.
type Companion struct {
	Producer string `json:"producer"` // the repository the pinned module is released from, owner/name
	Module   string `json:"module"`   // the pinned module's path
	Tag      string `json:"tag"`      // the release of it that goes with the pinning release
	Version  string `json:"version"`  // as the file requires it
	// RequiredBy is the pinning producer and its tag, whose file says so.
	RequiredBy string `json:"requiredBy"`
	File       string `json:"file"` // the file of the pinning release that was read
	// Error is why no release could be derived: the file is not there, does not
	// require the module, requires a version no release has, or one of another
	// stream. It stops the run.
	Error string `json:"error,omitempty"`
}

// pinSource is what deriving a companion reads from the host.
type pinSource interface {
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// pseudoVersion is the tail of a Go pseudo-version, which names a commit and no tag.
var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}(\+incompatible)?$`)

// derive finds the release of the pinned Artifact of a virtual Holding that goes
// with a release of the Artifact that pins it: it reads the pinning release's file
// at its tag from the host, takes the version of the pinned module there, and makes
// it the tag of the pinned Artifact's stream, which must exist.
func derive(ctx context.Context, src pinSource, v *chain.Holding, pinnerTag string) Companion {
	pinner, pinned := v.PinnedBy().Artifact(), v.Artifact()
	pinnerSlug := pinner.Stream().Project().Slug()
	c := Companion{
		Producer: v.Producer().Slug(), Module: pinned.Ref(),
		RequiredBy: pinnerSlug + " " + pinnerTag, File: pinner.PinFile(),
	}
	data, err := src.UpstreamFile(ctx, pinnerSlug, pinnerTag, c.File)
	if err != nil {
		c.Error = fmt.Sprintf("read %s of %s at %s: %v", c.File, pinnerSlug, pinnerTag, err)
		return c
	}
	if c.Version, err = gomodedit.Get(data, c.Module); err != nil {
		if errors.Is(err, gomodedit.ErrNotRequired) {
			c.Error = fmt.Sprintf("%s at %s does not require %s in %s", pinnerSlug, pinnerTag, c.Module, c.File)
		} else {
			c.Error = fmt.Sprintf("read %s of %s at %s: %v", c.File, pinnerSlug, pinnerTag, err)
		}
		return c
	}
	if pseudoVersion.MatchString(c.Version) {
		c.Error = fmt.Sprintf("%s requires %s %s, a pseudo-version, which no release tag has", c.RequiredBy, c.Module, c.Version)
		return c
	}
	stream := pinned.Stream()
	tag := tagPrefix(stream.TagPattern()) + c.Version
	if !stream.TagMatches(tag) {
		c.Error = fmt.Sprintf("%s requires %s %s, and %s is not a release of the stream %s", c.RequiredBy, c.Module, c.Version, tag, stream.Name())
		return c
	}
	refs, err := src.UpstreamRefs(ctx, c.Producer)
	if err != nil {
		c.Error = fmt.Sprintf("read the tags of %s: %v", c.Producer, err)
		return c
	}
	if !slices.ContainsFunc(refs.Tags, func(t git.Tag) bool { return t.Name == tag }) {
		c.Error = fmt.Sprintf("%s requires %s %s, and %s has no tag %s", c.RequiredBy, c.Module, c.Version, c.Producer, tag)
		return c
	}
	c.Tag = tag
	return c
}

// tagPrefix is what a stream's tag pattern puts before the version: the pattern up
// to its last slash, as a Go module released from a subdirectory is tagged.
func tagPrefix(pattern string) string {
	if i := strings.LastIndex(pattern, "/"); i >= 0 {
		return pattern[:i+1]
	}
	return ""
}

// companion is a Companion and the virtual Holding it is the release of.
type companion struct {
	Companion
	holding *chain.Holding
}

// derivedKey is what a companion is found by: the virtual Holding, by name, and the
// tag of the release that pins it.
type derivedKey struct{ holding, release string }

// withCompanions gives each target the virtual Holdings that follow its holdings,
// and the tag each pin resolves for the release the target gets, and says what was
// derived, each once. A pin that cannot be resolved is in the list with its error and
// is not given to any target: the plan blocks on it. The specs given are not changed.
func (r resolution) withCompanions(ctx context.Context, src pinSource, specs []spec) ([]spec, []Companion) {
	out := slices.Clone(specs)
	var list []Companion
	for i, sp := range specs {
		companions := r.route.Companions(sp.target)
		if len(companions) == 0 {
			continue
		}
		target := sp.target
		target.Holdings = slices.Clone(sp.target.Holdings)
		if len(target.Holdings) == 0 {
			target.Holdings = slices.Clone(r.route.Holdings)
		}
		target.Pinned = map[*chain.Holding]string{}
		for _, v := range companions {
			key := derivedKey{v.Name(), sp.release.Tag}
			c, ok := r.derived[key]
			if !ok {
				c = companion{derive(ctx, src, v, sp.release.Tag), v}
				r.derived[key] = c
			}
			if !slices.ContainsFunc(list, func(o Companion) bool { return o.RequiredBy == c.RequiredBy && o.Module == c.Module }) {
				list = append(list, c.Companion)
			}
			if c.Error != "" {
				continue
			}
			target.Holdings = append(target.Holdings, v)
			target.Pinned[v] = c.Tag
		}
		out[i].target = target
	}
	return out, list
}

// companionOf is the companion a virtual Holding has for a release.
func (r resolution) companionOf(h *chain.Holding, release string) companion {
	return r.derived[derivedKey{h.Name(), release}]
}

// companionsToShip is the companions some target that needs a PR will write, each
// once, in the order the targets are: the releases whose images and modules are to be
// looked for.
func (r resolution) companionsToShip(plan Plan) []companion {
	var out []companion
	for _, tp := range plan.Targets {
		if tp.Status != StatusReady {
			continue
		}
		for _, hp := range tp.Holdings {
			if !hp.Pinned || hp.UpToDate {
				continue
			}
			c := r.derived[derivedKey{hp.Name, tp.Release}]
			if c.holding != nil && !slices.ContainsFunc(out, func(o companion) bool { return o.Producer == c.Producer && o.Tag == c.Tag }) {
				out = append(out, c)
			}
		}
	}
	return out
}
