package ship

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// resolution is everything about a run that is the same for every target branch:
// who ships to whom, how the consumer keeps the value, which release, and the
// consumer's refs. Which release goes to which branch is not here: with a
// producer that has release lines, each line has its own (specs).
type resolution struct {
	producer   config.Project
	route      updatedispatcher.Route
	invocation updatedispatcher.Invocation
	tags       []git.Tag // the producer's, from the host
	// committer is who the commits are made as when git's own configuration is not
	// to decide: the bot's, in CI. Empty leaves it to git.
	committer    Identity
	consumerRefs git.Refs
}

// resolve finds the producer, picks the route to ship down, binds the edge's
// params to the consumer's method and resolves the release from the producer's
// tags on the host.
func resolve(ctx context.Context, ws Workspace, in Input) (resolution, error) {
	producer, err := ws.Project(in.Project)
	if err != nil {
		return resolution{}, err
	}
	engine, engineErr := ws.Updates()
	if engine == nil {
		return resolution{}, engineErr
	}
	routes := engine.RoutesFrom(producer)
	if len(routes) == 0 && engineErr != nil {
		return resolution{}, engineErr // a method of the route is not usable; say why
	}
	route, err := pickRoute(producer, routes, in.To, in.Method)
	if err != nil {
		return resolution{}, err
	}
	inv, err := engine.Invoke(route)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{producer: producer, route: route, invocation: inv}

	producerRefs, err := ws.UpstreamRefs(ctx, producer.Repo.Name)
	if err != nil {
		return resolution{}, fmt.Errorf("read the tags of %s: %w", producer.Repo.Name, err)
	}
	res.tags = producerRefs.Tags
	consumerSlug := route.Consumer.Repo.Name
	if res.consumerRefs, err = ws.UpstreamRefs(ctx, consumerSlug); err != nil {
		return resolution{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	return res, nil
}

// spec is one target branch of the consumer, with the release that goes to it.
type spec struct {
	branch  string
	release Release
}

// relOf is what the strategy makes the value from, for a release.
func (r resolution) relOf(rel Release) updatedispatcher.Release {
	return updatedispatcher.Release{
		Tag: rel.Tag, Version: strings.TrimPrefix(rel.Tag, "v"),
		Image: r.producer.EffectiveImages()[0].Name, SourceRepo: r.producer.Repo.Name,
	}
}

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
// branch for the line each is in: the producer has release lines of its own, and
// the edge does not send every release to every branch.
func (r resolution) followsLines() bool {
	return !r.producer.IsUniversal() && r.route.Edge.AcceptLines(r.producer) == config.AcceptLinesAll
}

// consumerLine finds a line of the consumer by name, with or without the v.
func (r resolution) consumerLine(line string) (config.ProjectLine, bool) {
	line = withV(line)
	for _, l := range r.route.Consumer.Release.Lines {
		if strings.EqualFold(l.Line, line) {
			return l, true
		}
	}
	return config.ProjectLine{}, false
}

func withV(line string) string {
	if !strings.HasPrefix(strings.ToLower(line), "v") {
		return "v" + line
	}
	return line
}

// specs chooses the targets to check, each with its release.
//
// A producer whose releases follow the lines, given no release, ships each line
// the latest of its own series (a line the producer has no release in is left
// out, with a note, unless it was named). Otherwise there is one release for every
// target: the tag, or the latest of a line or of all. A named line is a target on
// the branch the consumer keeps it on. Without one the edge's lines decide, as the
// dispatcher works them out for the release's line, and its error is given the
// advice to name a line. A release that follows lines is only shipped to the line
// it is in (ErrLineMismatch).
func (r resolution) specs(in Input) (selection, error) {
	if in.Arg == "" && r.followsLines() {
		return r.eachLine(in)
	}
	release, err := ResolveRelease(r.producer, r.tags, in.Arg, in.Prerelease, r.route.Method.Accept)
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
			if r.followsLines() && !strings.EqualFold(canonical.Line, release.Line) {
				where := "no line of " + r.producer.Repo.Name
				if release.Line != "" {
					where = release.Line
				}
				return selection{}, fmt.Errorf("%w: %s is in %s, not %s", ErrLineMismatch, release.Tag, where, canonical.Line)
			}
			branch, _ := r.route.Consumer.BranchOfLine(canonical.Line)
			sel.add(branch, release)
		}
		return sel, nil
	}
	branches, err := r.route.BranchesFor(r.producer, release.Line)
	if err != nil {
		return selection{}, fmt.Errorf("%w: %v; name a line of %s with --line", ErrNoTargets, err, r.route.Consumer.Name)
	}
	var sel selection
	for _, b := range branches {
		sel.add(b, release)
	}
	return sel, nil
}

// add makes a target of the branch, unless it already is one.
func (s *selection) add(branch string, release Release) {
	if !slices.ContainsFunc(s.specs, func(sp spec) bool { return sp.branch == branch }) {
		s.specs = append(s.specs, spec{branch, release})
	}
}

// eachLine is specs for a producer whose releases follow the lines, when no
// release was asked for: the latest of the series of each line, for the lines
// named, or else the lines both the producer and the consumer have, in the
// producer's order.
func (r resolution) eachLine(in Input) (selection, error) {
	sel := selection{named: len(in.Lines) > 0}
	var lines []string
	if sel.named {
		for _, line := range in.Lines {
			canonical, ok := r.consumerLine(line)
			if !ok {
				return selection{}, r.noSuchLine(line)
			}
			lines = append(lines, canonical.Line)
		}
	} else {
		for _, l := range r.producer.Release.Lines {
			if _, ok := r.consumerLine(l.Line); ok {
				lines = append(lines, l.Line)
			}
		}
	}
	for _, line := range lines {
		release, err := ResolveRelease(r.producer, r.tags, line, in.Prerelease, r.route.Method.Accept)
		if err != nil {
			if sel.named {
				return selection{}, err
			}
			sel.notes = append(sel.notes, fmt.Sprintf("%s: %v", line, err))
			continue
		}
		branch, _ := r.route.Consumer.BranchOfLine(line)
		sel.add(branch, release)
	}
	if len(sel.specs) == 0 {
		return selection{}, fmt.Errorf("%w: %s has no release in any line that %s has; name a line of %s with --line",
			ErrNoTargets, r.producer.Repo.Name, r.route.Consumer.Repo.Name, r.route.Consumer.Name)
	}
	return sel, nil
}

// noSuchLine is the error for a line the consumer does not have: it lists the
// ones it does.
func (r resolution) noSuchLine(line string) error {
	consumer := r.route.Consumer
	var have []string
	for _, l := range consumer.Release.Lines {
		have = append(have, l.Line)
	}
	if len(have) == 0 {
		return fmt.Errorf("%w: %s has no release lines, so there is no line %s to ship to", ErrNoTargets, consumer.Repo.Name, line)
	}
	return fmt.Errorf("%w: %s has no release line %s; it has %s", ErrNoTargets, consumer.Repo.Name, line, strings.Join(have, ", "))
}

// pickRoute chooses the route to ship down: the one to the consumer named by to
// (a config name or a slug), and with the method named by method when the
// producer uses several of one consumer; the only route when there is just one.
func pickRoute(producer config.Project, routes []updatedispatcher.Route, to, method string) (updatedispatcher.Route, error) {
	var matches []updatedispatcher.Route
	for _, r := range routes {
		toOK := to == "" || strings.EqualFold(r.Consumer.Name, to) || strings.EqualFold(r.Consumer.Repo.Name, to)
		methodOK := method == "" || r.Method.Name == method
		if toOK && methodOK {
			matches = append(matches, r)
		}
	}
	describe := func(rs []updatedispatcher.Route) string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Consumer.Name + " (" + r.Method.Name + ")"
		}
		return strings.Join(out, ", ")
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		if len(routes) == 0 {
			return updatedispatcher.Route{}, fmt.Errorf("%w: %s ships to no project: it has no downstream edges", ErrNoConsumer, producer.Name)
		}
		return updatedispatcher.Route{}, fmt.Errorf("%w: %s has no route to %q: it ships to %s", ErrNoConsumer, producer.Name, strings.TrimSpace(to+" "+method), describe(routes))
	}
	return updatedispatcher.Route{}, fmt.Errorf("%w for %s: %s; choose with --to and --method", ErrAmbiguousConsumer, producer.Name, describe(matches))
}
