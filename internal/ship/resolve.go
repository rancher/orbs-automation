package ship

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
)

// resolution is everything about a run that is the same for every target branch:
// who ships to whom, how the consumer keeps the value, which release, and what
// the value should become.
type resolution struct {
	producer     config.Project
	consumer     config.Consumer
	strategyName string
	strategy     Strategy
	params       Params
	file         string
	release      Release
	want         string
	consumerRefs git.Refs
}

// resolve finds the producer, picks the consumer, looks up the strategy and
// resolves the release from the producer's tags on the host.
func resolve(ctx context.Context, ws Workspace, in Input) (resolution, error) {
	producer, consumers, err := ws.Consumers(in.Project)
	if err != nil {
		return resolution{}, err
	}
	consumer, err := pickConsumer(producer, consumers, in.To)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{
		producer: producer, consumer: consumer,
		strategyName: consumer.Source.Update.Strategy, params: Params(consumer.Source.Update.Params),
	}
	if res.strategy, err = LookupStrategy(res.strategyName); err != nil {
		return resolution{}, err
	}
	if res.file, err = res.strategy.File(res.params); err != nil {
		return resolution{}, fmt.Errorf("strategy %s: %w", res.strategyName, err)
	}

	producerRefs, err := ws.UpstreamRefs(ctx, producer.Repo.Name)
	if err != nil {
		return resolution{}, fmt.Errorf("read the tags of %s: %w", producer.Repo.Name, err)
	}
	if res.release, err = ResolveRelease(producer, producerRefs.Tags, in.Arg, in.Prerelease, consumer.Source.Accept.When); err != nil {
		return resolution{}, err
	}
	if res.want, err = res.strategy.Value(res.params, map[string]string{
		"image":      producer.EffectiveImages()[0].Name,
		"tag":        res.release.Tag,
		"version":    strings.TrimPrefix(res.release.Tag, "v"),
		"sourceRepo": producer.Repo.Name,
	}); err != nil {
		return resolution{}, fmt.Errorf("strategy %s: %w", res.strategyName, err)
	}

	consumerSlug := consumer.Project.Repo.Name
	if res.consumerRefs, err = ws.UpstreamRefs(ctx, consumerSlug); err != nil {
		return resolution{}, fmt.Errorf("read the branches of %s: %w", consumerSlug, err)
	}
	return res, nil
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
	consumer := r.consumer.Project
	if r.consumer.Source.AcceptLines(r.producer) == config.AcceptLinesUniversal {
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

// pickConsumer chooses the consumer to ship to: the one named by to, or the
// only one.
func pickConsumer(producer config.Project, consumers []config.Consumer, to string) (config.Consumer, error) {
	names := make([]string, len(consumers))
	for i, c := range consumers {
		names[i] = c.Project.Name
	}
	if to != "" {
		for _, c := range consumers {
			if strings.EqualFold(c.Project.Name, to) || strings.EqualFold(c.Project.Repo.Name, to) {
				return c, nil
			}
		}
		return config.Consumer{}, fmt.Errorf("%w: %q does not take updates from %s (it is taken by: %s)",
			ErrNoConsumer, to, producer.Name, strings.Join(names, ", "))
	}
	switch len(consumers) {
	case 0:
		return config.Consumer{}, fmt.Errorf("%w: no project takes updates from %s", ErrNoConsumer, producer.Name)
	case 1:
		return consumers[0], nil
	}
	return config.Consumer{}, fmt.Errorf("%w take updates from %s: %s; choose one with --to",
		ErrAmbiguousConsumer, producer.Name, strings.Join(names, ", "))
}
