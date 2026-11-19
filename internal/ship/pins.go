package ship

import (
	"context"
	"slices"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/wanted"
)

// Companion is a release shipped with the one asked for, because the Artifact of
// that release pins the Artifact of this one. A Companion with an Error stops the run.
type Companion = wanted.Companion

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
func (r resolution) withCompanions(ctx context.Context, src wanted.Source, specs []spec) ([]spec, []Companion) {
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
				c = companion{wanted.Derive(ctx, src, v, sp.release.Tag), v}
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

// companionsToShip is the companions some target that needs a PR for the release
// will write, each once, in the order the targets are: the releases whose artifacts
// are to be looked for with it.
func (r resolution) companionsToShip(plan Plan, release string) []companion {
	var out []companion
	for _, tp := range plan.Targets {
		if tp.Status != StatusReady || tp.Release != release {
			continue
		}
		for _, hp := range tp.Holdings {
			if !hp.Pinned {
				continue // whether the consumer holds it already or not, it goes out with the release
			}
			c := r.derived[derivedKey{hp.Name, tp.Release}]
			if c.holding != nil && !slices.ContainsFunc(out, func(o companion) bool { return o.Producer == c.Producer && o.Tag == c.Tag }) {
				out = append(out, c)
			}
		}
	}
	return out
}
