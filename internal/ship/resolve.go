package ship

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	// derived is the companion each virtual Holding has for each release shipped,
	// found once. It is shared by the copies of the resolution, so a run and its
	// check agree on what was derived.
	derived map[derivedKey]companion
}

// resolve finds the producer, picks the route to ship down, and resolves the
// release from the producer's tags on the host.
//
// A producer the active team does not own is refused, unless the consumer is
// named and is one the team owns or one that is shared: then the release is
// shipped as for an owned producer, except that the steps that run from the
// producer's repo are skipped (bump.Route.SkipProducerSteps).
func resolve(ctx context.Context, ws Reader, in Input) (resolution, error) {
	producer, err := ws.Project(in.Project)
	if err != nil {
		return resolution{}, err
	}
	foreign := !producer.Owned(in.Team)
	if foreign && in.To == "" {
		return resolution{}, notOwned(producer, in.Team)
	}
	engine, engineErr := ws.Bump()
	if engine == nil {
		return resolution{}, engineErr
	}
	routes := engine.RoutesFrom(producer)
	if len(routes) == 0 && engineErr != nil {
		return resolution{}, engineErr // a holding or an edit of the route is not usable; say why
	}
	routes, err = narrowStreams(producer, routes, in)
	if err != nil {
		return resolution{}, err
	}
	route, err := pickRoute(producer, routes, in.To)
	if err != nil {
		return resolution{}, err
	}
	if foreign {
		if !route.Consumer.Owned(in.Team) && !route.Consumer.Shared() {
			return resolution{}, notOwnedConsumer(producer, route.Consumer, in.Team)
		}
		route.SkipProducerSteps = true
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
	res := resolution{producer: route.Producer, route: route, scheme: scheme, stream: stream, derived: map[derivedKey]companion{}}
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
	in = r.lineArgument(in)
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
			canonical, ok := r.route.Consumer.LineID(line)
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

// lineArgument reads an argument that is a line of the consumer, and no tag the
// producer has, as --line: a stream that is universal has no lines of its own, so
// a Rancher line given as the release can only be the consumer's. The release is
// then the latest of the stream the route is for. A stream that lists its lines
// reads the argument as its own.
func (r resolution) lineArgument(in Input) Input {
	if in.Arg == "" || r.followsLines() || slices.ContainsFunc(r.tags, func(t git.Tag) bool { return t.Name == in.Arg }) {
		return in
	}
	canonical, ok := r.route.Consumer.LineID(in.Arg)
	if !ok {
		return in
	}
	in.Arg = ""
	if !slices.Contains(in.Lines, canonical) {
		in.Lines = append(slices.Clone(in.Lines), canonical)
	}
	return in
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
			canonical, ok := r.route.Consumer.LineID(line)
			if !ok {
				return selection{}, r.noSuchLine(line)
			}
			names = append(names, canonical)
		}
	} else {
		for _, line := range r.scheme.Lines(r.tags) {
			if _, ok := r.route.Consumer.LineID(line); ok {
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

// narrowStreams chooses between the routes of one consumer that holds several
// streams of the producer, one for each minor of a library for instance, which the
// line decides: the consumer's Holding for the line is of one stream. It leaves
// the routes as they are when the user named no consumer with several, and when the
// consumer has one route, and gives the routes of the stream otherwise:
//
//   - a tag given as the argument picks the routes whose stream it is a release of;
//   - the lines (--line, or the argument when it names a line of the consumer that
//     no stream of the producer lists itself) pick the routes with a holding for
//     every one of them.
//
// Without a tag or a line it stops, saying what the consumer holds each stream for.
func narrowStreams(producer *chain.Project, routes []bump.Route, in Input) ([]bump.Route, error) {
	var of []bump.Route // the routes of the consumer the user named
	for _, r := range routes {
		if in.To == "" || r.Consumer.Named(in.To) {
			of = append(of, r)
		}
	}
	if len(of) < 2 || slices.ContainsFunc(of, func(r bump.Route) bool { return r.Consumer != of[0].Consumer }) {
		return routes, nil // one route, or several consumers, which --to chooses between
	}
	consumer := of[0].Consumer
	narrowed, tagged := of, false
	if in.Arg != "" {
		byTag := slices.DeleteFunc(slices.Clone(of), func(r bump.Route) bool { return r.Stream == nil || !r.Stream.TagMatches(in.Arg) })
		narrowed, tagged = byTag, len(byTag) > 0
		if !tagged {
			narrowed = of
		}
	}
	lines := slices.Clone(in.Lines)
	if in.Arg != "" && !tagged && consumer.ListsLine(in.Arg) && !producer.StreamListsLine(in.Arg) {
		lines = append(lines, in.Arg)
	}
	if len(lines) == 0 && !tagged {
		return nil, fmt.Errorf("%w for %s: %s holds it as %s; name a line of %s (gero ship %s <line> --to %s) or a tag",
			ErrAmbiguousConsumer, producer.Name(), consumer.Name(), describeStreams(of), consumer.Name(), producer.Name(), consumer.Name())
	}
	for _, line := range lines {
		narrowed = slices.DeleteFunc(slices.Clone(narrowed), func(r bump.Route) bool { return !r.AppliesTo(line) })
	}
	switch len(narrowed) {
	case 1:
		return narrowed, nil
	case 0:
		if len(lines) > 1 && len(of) > 1 {
			for _, line := range lines {
				if slices.ContainsFunc(of, func(r bump.Route) bool { return r.AppliesTo(line) }) {
					continue
				}
				return nil, fmt.Errorf("%w: %s holds no stream of %s for %s; it holds %s", ErrNoTargets, consumer.Name(), producer.Name(), line, describeStreams(of))
			}
			return nil, fmt.Errorf("%w for %s: %s holds %s on different streams, and one run ships one stream: %s",
				ErrAmbiguousConsumer, producer.Name(), consumer.Name(), strings.Join(lines, " and "), describeStreams(of))
		}
		return nil, fmt.Errorf("%w: %s holds no stream of %s for %s; it holds %s", ErrNoTargets, consumer.Name(), producer.Name(), strings.Join(lines, ", "), describeStreams(of))
	}
	return nil, fmt.Errorf("%w for %s: %s holds more than one stream for %s: %s", ErrAmbiguousConsumer, producer.Name(), consumer.Name(), strings.Join(lines, ", "), describeStreams(narrowed))
}

// describeStreams says what a consumer holds each stream of its routes for: the
// stream and the lines of the consumer that a holding of it is for.
func describeStreams(routes []bump.Route) string {
	out := make([]string, len(routes))
	for i, r := range routes {
		name := r.Stream.Name()
		var held []string
		for _, line := range r.Consumer.Lines() {
			if r.AppliesTo(line) {
				held = append(held, line)
			}
		}
		if len(held) > 0 {
			name += " (" + strings.Join(held, ", ") + ")"
		}
		out[i] = name
	}
	return strings.Join(out, ", ")
}

// pickRoute chooses the route to ship down: the one to the consumer named by to
// (a config name, a slug or an alias); the only route when there is just one.
func pickRoute(producer *chain.Project, routes []bump.Route, to string) (bump.Route, error) {
	var matches []bump.Route
	for _, r := range routes {
		if to == "" || r.Consumer.Named(to) {
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

// notOwnedConsumer says why gero does not ship another team's release into a
// project the active team does not own.
func notOwnedConsumer(producer, consumer *chain.Project, team string) error {
	return fmt.Errorf("%w: %s is managed by %s, so gero only ships it into a project %s owns or one that is shared, and %s is neither",
		ErrNotOwned, producer.Name(), orUnspecified(producer.ManagedBy()), orTheActiveTeam(team), consumer.Name())
}

// orUnspecified is the team of a project, or a word for none.
func orUnspecified(team string) string {
	if team == "" {
		return "no team"
	}
	return team
}

// orTheActiveTeam is the team gero acts for, or a phrase for none.
func orTheActiveTeam(team string) string {
	if team == "" {
		return "the active team"
	}
	return team
}

// notOwned says why gero does not ship from a project the active team does not
// own, naming the team that manages it when the config does.
func notOwned(p *chain.Project, team string) error {
	active := "is not owned by " + team
	if team == "" {
		active = "is not owned by the active team"
	}
	if by := p.ManagedBy(); by != "" {
		return fmt.Errorf("%w: %s %s, so gero only observes it, and %s ships its releases", ErrNotOwned, p.Name(), active, by)
	}
	return fmt.Errorf("%w: %s %s, so gero only observes it", ErrNotOwned, p.Name(), active)
}
