package ship

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/strategyspec"
)

// resolution is everything about a run that is the same for every target branch:
// who ships to whom, how the consumer keeps the value, which release, and what
// the value should become.
type resolution struct {
	producer     config.Project
	link         config.Link // the edge, the consumer and the update method
	strategy     Strategy
	params       Params // the strategy's params, bound from the method and the edge
	file         string
	release      Release
	want         string
	consumerRefs git.Refs
}

// resolve finds the producer, picks the consumer, looks up the strategy and
// resolves the release from the producer's tags on the host.
func resolve(ctx context.Context, ws Workspace, in Input) (resolution, error) {
	producer, links, err := ws.Links(in.Project)
	if err != nil {
		return resolution{}, err
	}
	link, err := pickLink(producer, links, in.To, in.Method)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{producer: producer, link: link, params: bindParams(link)}
	strategyName := link.Method.Strategy
	if res.strategy, err = LookupStrategy(strategyName); err != nil {
		return resolution{}, fmt.Errorf("method %s: %w", link.Method.Name, err)
	}
	if res.file, err = res.strategy.File(res.params); err != nil {
		return resolution{}, fmt.Errorf("method %s: %w", link.Method.Name, err)
	}

	producerRefs, err := ws.UpstreamRefs(ctx, producer.Repo.Name)
	if err != nil {
		return resolution{}, fmt.Errorf("read the tags of %s: %w", producer.Repo.Name, err)
	}
	if res.release, err = ResolveRelease(producer, producerRefs.Tags, in.Arg, in.Prerelease, link.Method.Accept.When); err != nil {
		return resolution{}, err
	}
	if res.want, err = res.strategy.Value(res.params, map[string]string{
		"image":      producer.EffectiveImages()[0].Name,
		"tag":        res.release.Tag,
		"version":    strings.TrimPrefix(res.release.Tag, "v"),
		"sourceRepo": producer.Repo.Name,
	}); err != nil {
		return resolution{}, fmt.Errorf("method %s: %w", link.Method.Name, err)
	}

	consumerSlug := link.Consumer.Repo.Name
	if res.consumerRefs, err = ws.UpstreamRefs(ctx, consumerSlug); err != nil {
		return resolution{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	return res, nil
}

// bindParams gathers what a strategy and the steps see: the strategy's own
// defaults, then the defaults of the params the method offers, then the values
// the method fixes, then what the producer's edge passes. A producer cannot
// override a fixed value; validation already rejects an edge that tries.
func bindParams(link config.Link) Params {
	params := Params{}
	spec, _ := strategyspec.Lookup(link.Method.Strategy)
	for _, sp := range spec.Params {
		if sp.Default != "" {
			params[sp.Key] = sp.Default
		}
	}
	for _, ap := range link.Method.AllowedParams {
		if ap.Default != "" {
			params[ap.Key] = ap.Default
		}
	}
	for key, value := range link.Edge.Params {
		params[key] = value
	}
	for key, value := range link.Method.With {
		params[key] = value
	}
	return params
}

// branches returns the consumer branches to check, and whether the user named
// them. A named branch is the only one. Otherwise the release's accept.lines
// decides: a universal source goes to every release branch of the consumer, and
// one that follows lines goes to the consumer's branch for the line the release
// is in.
func (r resolution) branches(in Input) (branches []string, named bool, err error) {
	if in.Branch != "" {
		return []string{in.Branch}, true, nil
	}
	consumer := r.link.Consumer
	if r.link.Edge.AcceptLines(r.producer) == config.AcceptLinesUniversal {
		if branches = consumer.ReleaseBranches(); len(branches) == 0 {
			return nil, false, fmt.Errorf("%w: %s has no release branches; name one with --branch", ErrNoTargets, consumer.Repo.Name)
		}
		return branches, false, nil
	}
	line := r.release.Line
	if line == "" {
		return nil, false, fmt.Errorf("%w: %s is in none of the release lines of %s, so there is no branch to follow it to; name one with --branch",
			ErrNoTargets, r.release.Tag, r.producer.Repo.Name)
	}
	for _, l := range consumer.Release.Lines {
		if l.Line == line {
			branch, _ := consumer.Branch(l)
			return []string{branch}, false, nil
		}
	}
	return nil, false, fmt.Errorf("%w: %s has no release line %s, which %s is in; name a branch with --branch",
		ErrNoTargets, consumer.Repo.Name, line, r.release.Tag)
}

// pickLink chooses the route to ship down: the one to the consumer named by to
// (a config name or a slug), and with the method named by method when the
// producer uses several of one consumer; the only route when there is just one.
func pickLink(producer config.Project, links []config.Link, to, method string) (config.Link, error) {
	var matches []config.Link
	for _, l := range links {
		toOK := to == "" || strings.EqualFold(l.Consumer.Name, to) || strings.EqualFold(l.Consumer.Repo.Name, to)
		methodOK := method == "" || l.Method.Name == method
		if toOK && methodOK {
			matches = append(matches, l)
		}
	}
	describe := func(ls []config.Link) string {
		out := make([]string, len(ls))
		for i, l := range ls {
			out[i] = l.Consumer.Name + " (" + l.Method.Name + ")"
		}
		return strings.Join(out, ", ")
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		if len(links) == 0 {
			return config.Link{}, fmt.Errorf("%w: %s ships to no project: it has no downstream edges", ErrNoConsumer, producer.Name)
		}
		return config.Link{}, fmt.Errorf("%w: %s has no route to %q: it ships to %s", ErrNoConsumer, producer.Name, to+" "+method, describe(links))
	}
	return config.Link{}, fmt.Errorf("%w for %s: %s; choose with --to and --method", ErrAmbiguousConsumer, producer.Name, describe(matches))
}
