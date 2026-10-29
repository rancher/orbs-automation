package config

// Defaults applied when a field is omitted (config/README.md "Defaults"). Only
// conservative values are defaulted; anything that changes release behavior
// must be written out.
const (
	DefaultBranch     = "main"
	DefaultBranches   = BranchesSingle
	DefaultTagPattern = "v*"
	DefaultAcceptWhen = AcceptStable
	DefaultAcceptLine = AcceptLinesAll
)

// WithDefaults returns a copy of c with every omitted field that has a default
// filled in. c is not modified.
func (c Config) WithDefaults() Config {
	c.Projects = mapSlice(c.Projects, Project.withDefaults)
	c.Externals = mapSlice(c.Externals, External.withDefaults)
	return c
}

func (p Project) withDefaults() Project {
	p.Repo = p.Repo.withDefaults()
	p.Release.Branches = orDefault(p.Release.Branches, DefaultBranches)
	p.Release.TagPattern = orDefault(p.Release.TagPattern, DefaultTagPattern)
	p.Sources = mapSlice(p.Sources, Source.withDefaults)
	return p
}

func (x External) withDefaults() External {
	x.Repo = x.Repo.withDefaults()
	return x
}

func (s Source) withDefaults() Source {
	s.Accept.When = orDefault(s.Accept.When, DefaultAcceptWhen)
	s.Accept.Lines = orDefault(s.Accept.Lines, DefaultAcceptLine)
	return s
}

func (r Repo) withDefaults() Repo {
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
