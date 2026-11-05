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

// branches returns the consumer branches to check. For now the user names one.
func (r resolution) branches(in Input) ([]string, error) {
	if in.Branch == "" {
		return nil, fmt.Errorf("%w: the release branches of %s are %s",
			ErrBranchRequired, r.consumer.Project.Repo.Name, strings.Join(r.consumer.Project.ReleaseBranches(), ", "))
	}
	return []string{in.Branch}, nil
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
