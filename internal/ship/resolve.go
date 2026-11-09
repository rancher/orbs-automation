package ship

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/tagscheme"
)

// resolution is everything about a run that is the same for every target branch:
// who ships to whom, which holdings are written, which release, and the consumer's
// refs. Which release goes to which branch is not here: with a producer that has
// release lines, each line has its own (specs).
type resolution struct {
	producer *chain.Project
	route    bump.Route
	scheme   tagscheme.Scheme
	stream   *chain.Stream
	tags     []git.Tag // the producer's, from the host
	// committer is who the commits are made as when git's own configuration is not
	// to decide: the bot's, in CI. Empty leaves it to git.
	committer    Identity
	consumerRefs git.Refs
}

// resolve finds the producer, picks the route to ship down, and resolves the
// release from the producer's tags on the host.
func resolve(ctx context.Context, ws Reader, in Input) (resolution, error) {
	producer, err := ws.Project(in.Project)
	if err != nil {
		return resolution{}, err
	}
	if !producer.Owned() {
		return resolution{}, notOwned(producer)
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return resolution{}, engineErr
	}
	routes := engine.RoutesFrom(producer)
	if len(routes) == 0 && engineErr != nil {
		return resolution{}, engineErr // a holding or an edit of the route is not usable; say why
	}
	route, err := pickRoute(producer, routes, in.To)
	if err != nil {
		return resolution{}, err
	}
	return resolveRoute(ctx, ws, route)
}

// resolveRoute is resolve for a route that is already chosen: it reads the
// producer's tags and the consumer's branches from the host. It is how gero repo
// status traces every consumer of a project with the same pieces that ship one
// release to one.
func resolveRoute(ctx context.Context, ws Reader, route bump.Route) (resolution, error) {
	stream := route.Stream
	scheme, err := tagscheme.ForStream(stream)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{producer: route.Producer, route: route, scheme: scheme, stream: stream}
	producerRefs, err := ws.UpstreamRefs(ctx, route.Producer.Slug())
	if err != nil {
		return resolution{}, fmt.Errorf("read the tags of %s: %w", route.Producer.Slug(), err)
	}
	res.tags = producerRefs.Tags
	consumerSlug := route.Consumer.Slug()
	if res.consumerRefs, err = ws.UpstreamRefs(ctx, consumerSlug); err != nil {
		return resolution{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	return res, nil
}

// acceptsAll says whether every holding of the route takes every release, a
// prerelease too.
func acceptsAll(route bump.Route) bool {
	for _, h := range route.Holdings {
		if h.Doc.Spec.Holds.Accept != chain.AcceptAll {
			return false
		}
	}
	return true
}

// spec is one target branch of the consumer, with the release that goes to it.
type spec struct {
	target  bump.Target
	release Release
}

// branch is the branch of the target.
func (s spec) branch() string { return s.target.Branch }

// selection is the targets of a run, each with its release.
type selection struct {
	specs []spec
	// named says the user named the lines, so a target that cannot be read is an
	// error and not one failed target among several.
	named bool
	// notes are lines that were left out, and why.
	notes []string
}

// followsLines reports whether the releases of the producer go to the consumer's
// branch for the line each is in: the stream has release lines of its own, and
// does not send every release to every branch.
func (r resolution) followsLines() bool { return !r.stream.Universal() }

// consumerLine finds a line of the consumer by name, with or without the v.
func (r resolution) consumerLine(line string) (string, bool) {
	line = lines.Normalize(line)
	for _, l := range r.route.Consumer.Lines() {
		if strings.EqualFold(l, line) {
			return l, true
		}
	}
	return "", false
}

// specs chooses the targets to check, each with its release.
//
// A producer whose releases follow the lines, given no release, ships each line
// the latest of its own series (a line the producer has no release in is left
// out, with a note, unless it was named). Otherwise there is one release for every
// target: the tag, or the latest of a line or of all. A named line is a target on
// the branch the consumer keeps it on. Without one the holdings decide, as the
// engine works them out for the release's line, and its error is given the advice
// to name a line. A release that follows lines is only shipped to the line it is
// in (ErrLineMismatch).
func (r resolution) specs(in Input) (selection, error) {
	if in.Arg == "" && r.followsLines() {
		return r.eachLine(in)
	}
	release, err := ResolveRelease(r.scheme, r.producer.Slug(), r.tags, in.Arg, in.Prerelease, acceptsAll(r.route))
	if err != nil {
		return selection{}, err
	}
	if len(in.Lines) > 0 {
		sel := selection{named: true}
		for _, line := range in.Lines {
			canonical, ok := r.consumerLine(line)
			if !ok {
				return selection{}, r.noSuchLine(line)
			}
			if r.followsLines() && !strings.EqualFold(canonical, release.Line) {
				where := "no line of " + r.producer.Slug()
				if release.Line != "" {
					where = release.Line
				}
				return selection{}, fmt.Errorf("%w: %s is in %s, not %s", ErrLineMismatch, release.Tag, where, canonical)
			}
			targets, err := r.route.TargetsOn(canonical)
			if err != nil {
				return selection{}, fmt.Errorf("%w: %v", ErrNoTargets, err)
			}
			for _, t := range targets {
				sel.add(t, release)
			}
		}
		return sel, nil
	}
	targets, err := r.route.TargetsFor(release.Line)
	if err != nil {
		return selection{}, fmt.Errorf("%w: %v; name a line of %s with --line", ErrNoTargets, err, r.route.Consumer.Name())
	}
	var sel selection
	for _, t := range targets {
		sel.add(t, release)
	}
	return sel, nil
}

// add makes a target of the branch, unless it already is one.
func (s *selection) add(target bump.Target, release Release) {
	if !slices.ContainsFunc(s.specs, func(sp spec) bool { return sp.branch() == target.Branch }) {
		s.specs = append(s.specs, spec{target, release})
	}
}

// eachLine is specs for a producer whose releases follow the lines, when no
// release was asked for: the latest of the series of each line, for the lines
// named, or else the lines both the producer and the consumer have, in the
// producer's order.
func (r resolution) eachLine(in Input) (selection, error) {
	sel := selection{named: len(in.Lines) > 0}
	var names []string
	if sel.named {
		for _, line := range in.Lines {
			canonical, ok := r.consumerLine(line)
			if !ok {
				return selection{}, r.noSuchLine(line)
			}
			names = append(names, canonical)
		}
	} else {
		for _, line := range r.scheme.Lines(r.tags) {
			if _, ok := r.consumerLine(line); ok {
				names = append(names, line)
			}
		}
	}
	for _, line := range names {
		release, err := ResolveRelease(r.scheme, r.producer.Slug(), r.tags, line, in.Prerelease, acceptsAll(r.route))
		if err != nil {
			if sel.named {
				return selection{}, err
			}
			sel.notes = append(sel.notes, fmt.Sprintf("%s: %v", line, err))
			continue
		}
		targets, err := r.route.TargetsOn(line)
		if err != nil {
			if sel.named {
				return selection{}, fmt.Errorf("%w: %v", ErrNoTargets, err)
			}
			sel.notes = append(sel.notes, fmt.Sprintf("%s: %v", line, err))
			continue
		}
		for _, t := range targets {
			sel.add(t, release)
		}
	}
	if len(sel.specs) == 0 {
		return selection{}, fmt.Errorf("%w: %s has no release in any line that %s has; name a line of %s with --line",
			ErrNoTargets, r.producer.Slug(), r.route.Consumer.Slug(), r.route.Consumer.Name())
	}
	return sel, nil
}

// noSuchLine is the error for a line the consumer does not have: it lists the
// ones it does.
func (r resolution) noSuchLine(line string) error {
	consumer := r.route.Consumer
	have := consumer.Lines()
	if len(have) == 0 {
		return fmt.Errorf("%w: %s has no release lines, so there is no line %s to ship to", ErrNoTargets, consumer.Slug(), line)
	}
	return fmt.Errorf("%w: %s has no release line %s; it has %s", ErrNoTargets, consumer.Slug(), line, strings.Join(have, ", "))
}

// pickRoute chooses the route to ship down: the one to the consumer named by to
// (a config name or a slug); the only route when there is just one.
func pickRoute(producer *chain.Project, routes []bump.Route, to string) (bump.Route, error) {
	var matches []bump.Route
	for _, r := range routes {
		if to == "" || strings.EqualFold(r.Consumer.Name(), to) || strings.EqualFold(r.Consumer.Slug(), to) {
			matches = append(matches, r)
		}
	}
	describe := func(rs []bump.Route) string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Consumer.Name()
			if r.Stream != nil && slices.ContainsFunc(rs, func(o bump.Route) bool { return o.Consumer == r.Consumer && o.Stream != r.Stream }) {
				out[i] += " (stream " + r.Stream.Name() + ")" // a consumer that takes several streams
			}
		}
		return strings.Join(out, ", ")
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		if len(routes) == 0 {
			return bump.Route{}, fmt.Errorf("%w: %s ships to no project: no project holds a release of it", ErrNoConsumer, producer.Name())
		}
		return bump.Route{}, fmt.Errorf("%w: %s has no route to %q: it ships to %s", ErrNoConsumer, producer.Name(), to, describe(routes))
	}
	if slices.ContainsFunc(matches, func(r bump.Route) bool { return r.Consumer != matches[0].Consumer }) {
		return bump.Route{}, fmt.Errorf("%w for %s: %s; choose with --to", ErrAmbiguousConsumer, producer.Name(), describe(matches))
	}
	return bump.Route{}, fmt.Errorf("%w for %s: %s takes several of its streams, which ship cannot tell apart yet: %s", ErrAmbiguousConsumer, producer.Name(), matches[0].Consumer.Name(), describe(matches))
}

// notOwned says why gero does not ship from an unowned project, naming the team
// that does when the config does.
func notOwned(p *chain.Project) error {
	if team := p.Steward(); team != "" {
		return fmt.Errorf("%w: %s is not owned, so gero only observes it, and %s ships its releases", ErrNotOwned, p.Name(), team)
	}
	return fmt.Errorf("%w: %s is not owned, so gero only observes it", ErrNotOwned, p.Name())
}
