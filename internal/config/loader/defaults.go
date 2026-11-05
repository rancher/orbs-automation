package loader

import "github.com/rancher/orbs-automation/internal/config"

// Defaults applied when a field is omitted (config/README.md "Defaults"). Only
// conservative values are defaulted; anything that changes release behavior
// must be written out.
const (
	DefaultBranch     = "main"
	DefaultBranches   = config.BranchesSingle
	DefaultTagPattern = "v*"
	DefaultAcceptWhen = config.AcceptStable
)

// WithDefaults returns a copy of c with every omitted field that has a default
// filled in. c is not modified.
func WithDefaults(c config.Config) config.Config {
	c.Projects = mapSlice(c.Projects, projectDefaults)
	c.Externals = mapSlice(c.Externals, externalDefaults)
	return c
}

func projectDefaults(p config.Project) config.Project {
	p.Repo = repoDefaults(p.Repo)
	p.Release.Branches = orDefault(p.Release.Branches, DefaultBranches)
	p.Release.TagPattern = orDefault(p.Release.TagPattern, DefaultTagPattern)
	p.Sources = mapSlice(p.Sources, sourceDefaults)
	p.UpdateMethods = mapSlice(p.UpdateMethods, methodDefaults)
	return p
}

func externalDefaults(x config.External) config.External {
	x.Repo = repoDefaults(x.Repo)
	return x
}

func methodDefaults(m config.UpdateMethod) config.UpdateMethod {
	m.Accept.When = orDefault(m.Accept.When, DefaultAcceptWhen)
	return m
}

func sourceDefaults(s config.Source) config.Source {
	s.Accept.When = orDefault(s.Accept.When, DefaultAcceptWhen)
	return s
}

func repoDefaults(r config.Repo) config.Repo {
	r.DefaultBranch = orDefault(r.DefaultBranch, DefaultBranch)
	return r
}

// mapSlice returns a new slice of f applied to each element, so the caller's
// backing array is never shared. A nil slice stays nil.
func mapSlice[T any](in []T, f func(T) T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
