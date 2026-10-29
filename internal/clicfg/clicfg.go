// Package clicfg resolves the CLI config of one gero invocation (flags, GERO_*
// environment, the user file and defaults) into an immutable Settings value.
// It is framework-free (no cobra) and does no git inference: unset stays empty
// and a resolver on top of Settings fills it in from the working directory.
// See docs/specs/CLI_CONFIG.md.
package clicfg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	OutputEnv  = "env" // KEY=value lines for $GITHUB_OUTPUT and $GITHUB_ENV
)

// Log levels, lowest verbosity last.
const (
	LogDebug = "debug"
	LogInfo  = "info"
	LogWarn  = "warn"
	LogError = "error"
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
	LogLevel     string // LogDebug, LogInfo, LogWarn or LogError
	Upstream     string // remote treated as the real source
	Fork         string // remote holding the developer's fork
	GitHubUser   string
	BranchPrefix string
	Repos        Repos
	UserFile     string // path of the user file that was loaded; empty when none applied
	CI           bool   // the process runs in CI rather than on a workstation
	BotUser      string // GERO_BOT_USER: the bot's login, from the workflow; env only

	sources map[string]Source
}

// Source reports the layer key (a user-file key such as "configDir") came from.
// Keys that only exist in the file report SourceFile when set.
func (s Settings) Source(key string) Source {
	return s.sources[key]
}

// scalar is a key that can come from a flag, the environment or the file.
type scalar struct {
	name     string // file key; flag and env names derive from it
	def      string
	path     bool // expand "~" and $VAR; file values must then be absolute
	validate func(string) error
	field    func(*Settings) *string
	fromFile func(*fileConfig) string
}

var scalars = []scalar{
	{
		name:     "configDir",
		path:     true,
		field:    func(s *Settings) *string { return &s.ConfigDir },
		fromFile: func(f *fileConfig) string { return f.ConfigDir },
	},
	{
		name:     "output",
		def:      OutputText,
		validate: validateOutput,
		field:    func(s *Settings) *string { return &s.Output },
		fromFile: func(f *fileConfig) string { return f.Output },
	},
	{
		// Flag and env only: -v and -vv map onto it in cmd/, and there is no
		// user-file key.
		name:     "log",
		def:      LogWarn,
		validate: validateLog,
		field:    func(s *Settings) *string { return &s.LogLevel },
		fromFile: func(*fileConfig) string { return "" },
	},
	{
		name:     "upstream",
		field:    func(s *Settings) *string { return &s.Upstream },
		fromFile: func(f *fileConfig) string { return f.Upstream },
	},
}

func validateOutput(v string) error {
	if v != OutputText && v != OutputJSON && v != OutputEnv {
		return fmt.Errorf("must be %q, %q or %q, got %q", OutputText, OutputJSON, OutputEnv, v)
	}
	return nil
}

func validateLog(v string) error {
	switch v {
	case LogDebug, LogInfo, LogWarn, LogError:
		return nil
	}
	return fmt.Errorf("must be %q, %q, %q or %q, got %q", LogDebug, LogInfo, LogWarn, LogError, v)
}

// Load resolves Settings. Precedence per key: flag, env, user file, default.
func Load(in Inputs) (Settings, error) {
	getenv := in.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	fc, err := decodeFile(in.UserFile, in.UserFileName)
	if err != nil {
		return Settings{}, err
	}
	s := Settings{sources: map[string]Source{}}
	if s.CI, err = InCI(getenv); err != nil {
		return Settings{}, err
	}
	label := userFileLabel(in.UserFileName)
	// Record the file only when one was read: a name with no reader is where we
	// looked, not a file that was loaded. GERO_CONFIG is the one way to name it.
	if in.UserFile != nil {
		s.UserFile = in.UserFileName
	}
	s.sources["userFile"] = SourceDefault
	if getenv("GERO_CONFIG") != "" {
		s.sources["userFile"] = SourceEnv
	}

	for _, k := range scalars {
		v, src := k.def, SourceDefault
		if fv := k.fromFile(&fc); fv != "" {
			v, src = fv, SourceFile
		}
		if ev := getenv(EnvName(k.name)); ev != "" {
			v, src = ev, SourceEnv
		}
		if in.Flags != nil {
			if fv, set := in.Flags(FlagName(k.name)); set {
				v, src = fv, SourceFlag
			}
		}
		if k.path && src != SourceDefault {
			if v, err = expandPath(v, in.Home, getenv); err != nil {
				return Settings{}, fmt.Errorf("%s: %w", where(k.name, src, label), err)
			}
			if src == SourceFile && !filepath.IsAbs(v) {
				return Settings{}, fmt.Errorf("%s: path %q must be absolute", where(k.name, src, label), v)
			}
		}
		if k.validate != nil {
			if err := k.validate(v); err != nil {
				return Settings{}, fmt.Errorf("%s: %w", where(k.name, src, label), err)
			}
		}
		*k.field(&s) = v
		s.sources[k.name] = src
	}

	s.Fork, s.GitHubUser, s.BranchPrefix = fc.Fork, fc.GitHubUser, fc.BranchPrefix
	for key, v := range map[string]string{"fork": fc.Fork, "githubUser": fc.GitHubUser, "branchPrefix": fc.BranchPrefix} {
		if v != "" {
			s.sources[key] = SourceFile
		}
	}
	if s.Repos, err = resolveRepos(fc.Repos, in.Home, getenv); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", label, err) // the repos keys exist only in the file
	}
	return s, nil
}

// where says what a rejected value came from, so the message sends the developer
// to the right place: the user file and key, or the key and the environment
// variable or flag that set it.
func where(key string, src Source, fileLabel string) string {
	if src == SourceFile {
		return fileLabel + ": " + key
	}
	return fmt.Sprintf("%s (from %s)", key, src)
}

var slugRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func resolveRepos(r fileRepos, home string, getenv func(string) string) (Repos, error) {
	out := Repos{Overrides: map[string]RepoOverride{}, Paths: map[string]string{}}
	for _, dir := range r.Search {
		p, err := absPath("repos.search", dir, home, getenv)
		if err != nil {
			return Repos{}, err
		}
		out.Search = append(out.Search, p)
	}
	for slug, dir := range r.Paths {
		if !slugRE.MatchString(slug) {
			return Repos{}, fmt.Errorf("repos.paths: %q is not an owner/name slug", slug)
		}
		p, err := absPath("repos.paths."+slug, dir, home, getenv)
		if err != nil {
			return Repos{}, err
		}
		out.Paths[slug] = p
	}
	for slug, o := range r.Overrides {
		if !slugRE.MatchString(slug) {
			return Repos{}, fmt.Errorf("repos.overrides: %q is not an owner/name slug", slug)
		}
		out.Overrides[slug] = RepoOverride{Upstream: o.Upstream}
	}
	return out, nil
}

func absPath(key, p, home string, getenv func(string) string) (string, error) {
	p, err := expandPath(p, home, getenv)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%s: path %q must be absolute", key, p)
	}
	return filepath.Clean(p), nil
}

// expandPath replaces a leading "~" with home and expands $VAR and ${VAR}.
func expandPath(p, home string, getenv func(string) string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", fmt.Errorf("cannot expand %q: no home directory", p)
		}
		p = home + p[1:]
	}
	return os.Expand(p, getenv), nil
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
