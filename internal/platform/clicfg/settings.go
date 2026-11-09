package clicfg

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

// RepoOverride holds per-repo settings from repos.overrides.
type RepoOverride struct {
	Upstream string
	PushTo   string // fork or upstream: where branches for this repo are pushed
}

// Repos locates local clones of the repositories named in the central config.
type Repos struct {
	Search    []string                // directories scanned for clones that exist
	CloneRoot string                  // where new clones go, as <root>/<name>; never taken from Search
	Paths     map[string]string       // slug (owner/name) to clone path
	Overrides map[string]RepoOverride // slug to per-repo settings
}

// Settings is the resolved CLI config. Treat it as read-only and pass it down by
// value.
type Settings struct {
	CentralConfigDir string // central application config directory
	Output           string // OutputText or OutputJSON
	Color            string // ColorAuto, ColorAlways or ColorNever
	Terminal         Terminal
	// ReplPrompt is the template of the prompt of gero repl, repl.prompt in the
	// user file. It is checked when the REPL starts and not here, so a bad one
	// breaks the REPL and no other command.
	ReplPrompt   string
	LogLevel     string // LogDebug, LogInfo, LogWarn or LogError
	Upstream     string // remote treated as the real source
	Fork         string // remote holding the developer's fork
	GitHubUser   string
	BranchPrefix string
	PushTo       string // fork or upstream: where branches are pushed, unless a repo or the flag says
	Repos        Repos
	UserFile     string // path of the user file that was loaded; empty when none applied
	// Warnings are what was wrong with the user file and did not stop it loading,
	// such as a key gero does not know, each naming the file and the line.
	Warnings []string
	CI       bool   // the process runs in CI rather than on a workstation
	BotUser  string // GERO_BOT_USER: the bot's login, from the workflow; env only
	// DryRun is --dry-run or GERO_DRY_RUN: a command that writes computes and
	// prints what it would do, and changes nothing. There is no user-file key, so
	// it is never on without being asked for in the invocation.
	DryRun bool
	// NonInteractive is --non-interactive (or --yes) or GERO_NON_INTERACTIVE: a
	// write command goes on without asking its one question. There is no
	// user-file key, so it is never on without being asked for in the invocation.
	NonInteractive bool
	// GeroVersion is the running gero's release, such as v0.5.0 or "dev", set by
	// cmd/ and never by a file or the environment. Empty means no release context:
	// the central config bundle is not looked for (docs/specs/CENTRAL_BUNDLE.md).
	GeroVersion string
	// Offline is GERO_OFFLINE: a command makes no request to the network where it
	// has a local way, and the central config bundle is read from the cache only.
	// There is no user-file key, so it is never on without being asked for.
	Offline bool
	// SignCommits is signCommits in the user file or GERO_SIGN_COMMITS: the
	// commits a command makes are signed by git, with the engineer's own key.
	SignCommits bool
	// DefaultToken is true when the token in the environment is the workflow's
	// default GITHUB_TOKEN: GITHUB_TOKEN is set and GH_TOKEN is unset or equal
	// to it. Only the fact is kept, never a token.
	DefaultToken bool

	sources map[string]Source
}

// Source reports the layer key (a user-file key such as "centralConfigDir") came from.
// Keys that only exist in the file report SourceFile when set.
func (s Settings) Source(key string) Source {
	return s.sources[key]
}
