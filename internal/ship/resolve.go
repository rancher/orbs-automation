package ship

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// resolution is everything about a run that is the same for every target branch:
// who ships to whom, how the consumer keeps the value, which release, and the
// consumer's refs.
type resolution struct {
	producer     config.Project
	route        updatedispatcher.Route
	invocation   updatedispatcher.Invocation
	release      Release
	rel          updatedispatcher.Release // what the strategy makes the value from
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
	if res.release, err = ResolveRelease(producer, producerRefs.Tags, in.Arg, in.Prerelease, route.Method.Accept); err != nil {
		return resolution{}, err
	}
	res.rel = updatedispatcher.Release{
		Tag: res.release.Tag, Version: strings.TrimPrefix(res.release.Tag, "v"),
		Image: producer.EffectiveImages()[0].Name, SourceRepo: producer.Repo.Name,
	}
	consumerSlug := route.Consumer.Repo.Name
	if res.consumerRefs, err = ws.UpstreamRefs(ctx, consumerSlug); err != nil {
		return resolution{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	return res, nil
}

// branches returns the consumer branches to check, and whether the user named
// them. A named line is the only one, on the branch the consumer keeps it on.
// Otherwise the edge's lines decide, as the dispatcher works them out for the
// release's line, and its error is given the advice to name a line.
func (r resolution) branches(in Input) (branches []string, named bool, err error) {
	if in.Line != "" {
		branch, err := r.lineBranch(in.Line)
		if err != nil {
			return nil, true, err
		}
		return []string{branch}, true, nil
	}
	branches, err = r.route.BranchesFor(r.producer, r.release.Line)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v; name a line of %s with --line", ErrNoTargets, err, r.route.Consumer.Name)
	}
	return branches, false, nil
}

// lineBranch is the branch the consumer keeps a line on: its own, or the default
// branch for a line that has none (the one in development). The line may be
// written without the v. A line the consumer does not have is an error that
// lists the ones it does.
func (r resolution) lineBranch(line string) (string, error) {
	consumer := r.route.Consumer
	want := line
	if !strings.HasPrefix(want, "v") {
		want = "v" + want
	}
	var have []string
	for _, l := range consumer.Release.Lines {
		if l.Line == want {
			branch, _ := consumer.Branch(l)
			return branch, nil
		}
		have = append(have, l.Line)
	}
	if len(have) == 0 {
		return "", fmt.Errorf("%w: %s has no release lines, so there is no line %s to ship to", ErrNoTargets, consumer.Repo.Name, line)
	}
	return "", fmt.Errorf("%w: %s has no release line %s; it has %s", ErrNoTargets, consumer.Repo.Name, line, strings.Join(have, ", "))
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
