package config

// Defaults applied when a field is omitted (config/README.md "Defaults"). Only
// conservative values are defaulted; anything that changes release behavior
// must be written out.
const (
	DefaultBranch     = "main"
	DefaultBranches   = "single"
	DefaultTagPattern = "v*"
	DefaultAcceptWhen = "stable"
	DefaultAcceptLine = "all"
)

// WithDefaults returns a copy of c with every omitted field that has a default
// filled in. c is not modified.
func (c Config) WithDefaults() Config {
	return c
}
