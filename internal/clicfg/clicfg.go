// Package clicfg resolves the CLI config of one gero invocation (flags, GERO_*
// environment, the user file and defaults) into an immutable Settings value.
// It is framework-free (no cobra) and does no git inference: unset stays empty
// and a resolver on top of Settings fills it in from the working directory.
// See docs/specs/CLI_CONFIG.md.
package clicfg

import (
	"io"
	"strings"
	"unicode"
)

// Source says which layer a Settings value came from.
type Source int

// The layers, lowest precedence first.
const (
	SourceDefault Source = iota
	SourceFile
	SourceEnv
	SourceFlag
)

func (s Source) String() string {
	switch s {
	case SourceFile:
		return "file"
	case SourceEnv:
		return "env"
	case SourceFlag:
		return "flag"
	default:
		return "default"
	}
}

// Output formats.
const (
	OutputText = "text"
	OutputJSON = "json"
)

// FlagLookup reports the value of the flag named name and whether it was set
// explicitly. cmd/ builds it over cobra, where set is Flags().Changed, so a
// flag's default never masks a lower layer.
type FlagLookup func(name string) (value string, set bool)

// Inputs are everything Load reads. Nothing else (no os.Getenv, no file system)
// is touched, so the loader is table-testable.
type Inputs struct {
	Flags        FlagLookup          // nil: no flags set
	Getenv       func(string) string // nil: empty environment
	UserFile     io.Reader           // nil: no user file
	UserFileName string              // used in error messages only
	Home         string              // expands a leading "~" in paths
}

// RepoOverride holds per-repo settings from repos.overrides.
type RepoOverride struct {
	Upstream string
}

// Repos locates local clones of the repositories named in the central config.
type Repos struct {
	Search    []string                // directories scanned for clones
	Paths     map[string]string       // slug (owner/name) to clone path
	Overrides map[string]RepoOverride // slug to per-repo settings
}

// Settings is the resolved CLI config. Treat it as read-only and pass it down by
// value.
type Settings struct {
	ConfigDir    string // central application config directory
	Output       string // OutputText or OutputJSON
	Upstream     string // remote treated as the real source
	Fork         string // remote holding the developer's fork
	GitHubUser   string
	BranchPrefix string
	Repos        Repos

	sources map[string]Source
}

// Source reports the layer key (a user-file key such as "configDir") came from.
// Keys that only exist in the file report SourceFile when set.
func (s Settings) Source(key string) Source {
	return s.sources[key]
}

// scalar is a key that can come from a flag, the environment or the file.
type scalar struct {
	name  string // file key; flag and env names derive from it
	def   string
	field func(*Settings) *string
}

var scalars = []scalar{
	{name: "configDir", field: func(s *Settings) *string { return &s.ConfigDir }},
	{name: "output", def: OutputText, field: func(s *Settings) *string { return &s.Output }},
	{name: "upstream", field: func(s *Settings) *string { return &s.Upstream }},
}

// Load resolves Settings. Precedence per key: flag, env, user file, default.
func Load(in Inputs) (Settings, error) {
	getenv := in.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	s := Settings{sources: map[string]Source{}}
	for _, k := range scalars {
		v, src := k.def, SourceDefault
		if ev := getenv(EnvName(k.name)); ev != "" {
			v, src = ev, SourceEnv
		}
		if in.Flags != nil {
			if fv, set := in.Flags(FlagName(k.name)); set {
				v, src = fv, SourceFlag
			}
		}
		*k.field(&s) = v
		s.sources[k.name] = src
	}
	return s, nil
}

// EnvName derives the environment variable for a file key:
// configDir -> GERO_CONFIG_DIR.
func EnvName(key string) string {
	return "GERO_" + strings.ToUpper(splitWords(key, '_'))
}

// FlagName derives the flag for a file key: configDir -> config-dir.
func FlagName(key string) string {
	return strings.ToLower(splitWords(key, '-'))
}

func splitWords(key string, sep rune) string {
	var b strings.Builder
	for i, r := range key {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteRune(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}
