package clicfg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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
	Facts        Facts               // what the process knows of its terminal
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
		// The template of the prompt of gero repl: a user-file key, repl.prompt, and
		// an environment variable, GERO_REPL_PROMPT. There is no flag, since a
		// prompt is not a per-invocation choice.
		name:     "repl.prompt",
		def:      DefaultReplPrompt,
		field:    func(s *Settings) *string { return &s.ReplPrompt },
		fromFile: func(f *fileConfig) string { return f.Repl.Prompt },
	},
	{
		name:     "color",
		def:      ColorAuto,
		validate: validateColor,
		field:    func(s *Settings) *string { return &s.Color },
		fromFile: func(f *fileConfig) string { return f.Color },
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

func validateColor(v string) error {
	if v != ColorAuto && v != ColorAlways && v != ColorNever {
		return fmt.Errorf("must be %q, %q or %q, got %q", ColorAuto, ColorAlways, ColorNever, v)
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
	s.BotUser = getenv("GERO_BOT_USER")
	s.DefaultToken = getenv("GITHUB_TOKEN") != "" && (getenv("GH_TOKEN") == "" || getenv("GH_TOKEN") == getenv("GITHUB_TOKEN"))
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

	s.Terminal = ResolveTerminal(s.Color, getenv, in.Facts)

	if s.DryRun, s.sources["dryRun"], err = loadBool("dryRun", in.Flags, getenv, "dry-run"); err != nil {
		return Settings{}, err
	}
	// --yes is the same switch as --non-interactive.
	if s.NonInteractive, s.sources["nonInteractive"], err = loadBool("nonInteractive", in.Flags, getenv, "non-interactive", "yes"); err != nil {
		return Settings{}, err
	}
	s.sources["signCommits"] = SourceDefault
	if fc.SignCommits != nil {
		s.SignCommits, s.sources["signCommits"] = *fc.SignCommits, SourceFile
	}
	if ev := getenv(EnvName("signCommits")); ev != "" {
		on, err := strconv.ParseBool(ev)
		if err != nil {
			return Settings{}, fmt.Errorf("%s: must be true or false, got %q", where("signCommits", SourceEnv, ""), ev)
		}
		s.SignCommits, s.sources["signCommits"] = on, SourceEnv
	}

	s.Fork, s.GitHubUser, s.BranchPrefix, s.PushTo = fc.Fork, fc.GitHubUser, fc.BranchPrefix, fc.PushTo
	if err := checkRole("pushTo", fc.PushTo); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", label, err)
	}
	for key, v := range map[string]string{"fork": fc.Fork, "githubUser": fc.GitHubUser, "branchPrefix": fc.BranchPrefix, "pushTo": fc.PushTo} {
		if v != "" {
			s.sources[key] = SourceFile
		}
	}
	if s.Repos, err = resolveRepos(fc.Repos, in.Home, getenv); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", label, err) // the repos keys exist only in the file
	}
	return s, nil
}

// flagValue reads a flag through the lookup, which may be nil.
func flagValue(flags FlagLookup, name string) (string, bool) {
	if flags == nil {
		return "", false
	}
	return flags(name)
}

// loadBool reads an on/off switch that is a flag (under any of flagNames) or an
// environment variable and never a user-file key: the last flag given, else the
// environment, else off. It says which layer decided. A file that could turn
// these on would silence every write or answer yes to every question.
func loadBool(key string, flags FlagLookup, getenv func(string) string, flagNames ...string) (bool, Source, error) {
	value, src := "", SourceDefault
	if v := getenv(EnvName(key)); v != "" {
		value, src = v, SourceEnv
	}
	for _, name := range flagNames {
		if v, set := flagValue(flags, name); set {
			value, src = v, SourceFlag
		}
	}
	if value == "" {
		return false, src, nil
	}
	on, err := strconv.ParseBool(value)
	if err != nil {
		return false, src, fmt.Errorf("%s: must be true or false, got %q", where(key, src, ""), value)
	}
	return on, src, nil
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

// checkRole accepts an unset push target, and fork and upstream.
func checkRole(key, value string) error {
	if value != "" && value != "fork" && value != "upstream" {
		return fmt.Errorf("%s: %q is not fork or upstream", key, value)
	}
	return nil
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
	if r.CloneRoot != "" {
		p, err := absPath("repos.cloneRoot", r.CloneRoot, home, getenv)
		if err != nil {
			return Repos{}, err
		}
		out.CloneRoot = p
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
		if err := checkRole("repos.overrides."+slug+".pushTo", o.PushTo); err != nil {
			return Repos{}, err
		}
		out.Overrides[slug] = RepoOverride{Upstream: o.Upstream, PushTo: o.PushTo}
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
// configDir -> GERO_CONFIG_DIR, repl.prompt -> GERO_REPL_PROMPT.
func EnvName(key string) string {
	return "GERO_" + strings.ToUpper(splitWords(key, '_'))
}

// FlagName derives the flag for a file key: configDir -> config-dir.
func FlagName(key string) string {
	return strings.ToLower(splitWords(key, '-'))
}

// splitWords puts sep between the words of a key: before a capital, and where a
// dot nests the key (repl.prompt).
func splitWords(key string, sep rune) string {
	var b strings.Builder
	for i, r := range key {
		switch {
		case r == '.':
			b.WriteRune(sep)
			continue
		case i > 0 && unicode.IsUpper(r):
			b.WriteRune(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}
