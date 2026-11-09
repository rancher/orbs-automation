package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/cli/configresolve"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what the configuration checks need from the invocation's
// workspace, the one door to the central config and the local clones
// (docs/specs/WORKSPACE.md). It is defined here, where it is consumed, so tests
// can use a fake; *workspace.Workspace satisfies it.
type Workspace interface {
	ConfigSource() (workspace.ConfigSource, error)
	Chain() (*chain.Graph, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// configDirCheck finds what the central config is read from and says where it came
// from: a local directory, or the central bundle with its release and age.
type configDirCheck struct{ ws Workspace }

// NewConfigDirCheck returns the check named "central-config-dir".
func NewConfigDirCheck(ws Workspace) Check { return configDirCheck{ws: ws} }

func (configDirCheck) Name() string     { return "central-config-dir" }
func (configDirCheck) Category() string { return "configuration" }
func (configDirCheck) Description() string {
	return "Finds the central config, a directory or the central bundle, and says where it came from"
}
func (configDirCheck) Online() bool    { return false }
func (configDirCheck) Needs() []string { return nil }

func (c configDirCheck) Run(context.Context) Result {
	src, err := c.ws.ConfigSource()
	switch {
	case errors.Is(err, workspace.ErrNotFound):
		remediation := workspace.NotFoundHint + "; or run gero doctor --online to fetch the central config bundle"
		if err != workspace.ErrNotFound {
			// The bundle was tried and failed, and the rest of the error says why.
			remediation = workspace.NotFoundHint + strings.TrimPrefix(err.Error(), workspace.ErrNotFound.Error())
		}
		return Result{Status: StatusFail, Message: "no central config directory found", Remediation: remediation}
	case err != nil:
		return Result{Status: StatusFail, Message: err.Error()}
	case src.Bundle != nil && src.Bundle.Stale:
		return Result{Status: StatusWarn, Message: src.Bundle.Describe(), Remediation: src.Bundle.Note}
	case src.Bundle != nil:
		return Result{Status: StatusOK, Message: src.Bundle.Describe()}
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("%s (%s)", src.Dir.Dir, src.Dir.Source)}
}

// configCheck loads the central application config and reports whether it is
// usable. It needs central-config-dir.
type configCheck struct{ ws Workspace }

// NewConfigCheck returns the check named "config".
func NewConfigCheck(ws Workspace) Check { return configCheck{ws: ws} }

func (configCheck) Name() string        { return "config" }
func (configCheck) Category() string    { return "configuration" }
func (configCheck) Description() string { return "Loads the central config and reports any problems" }
func (configCheck) Online() bool        { return false }
func (configCheck) Needs() []string     { return []string{"central-config-dir"} }

func (c configCheck) Run(context.Context) Result {
	g, err := c.ws.Chain()
	if err != nil {
		n := problemCount(err)
		return Result{
			Status:      StatusFail,
			Message:     fmt.Sprintf("%d %s in the central config", n, plural(n, "problem")),
			Remediation: validateHint,
		}
	}
	var streams, artifacts int
	for _, p := range g.Projects() {
		streams += len(p.Streams())
		artifacts += len(p.Artifacts())
	}
	holdings, lines := len(g.Holdings()), len(g.Lines())
	msg := fmt.Sprintf("%d %s, %d %s, %d %s, %d %s, %d %s",
		len(g.Projects()), plural(len(g.Projects()), "project"),
		streams, plural(streams, "stream"),
		artifacts, plural(artifacts, "artifact"),
		holdings, plural(holdings, "holding"),
		lines, plural(lines, "line"))
	if n := len(g.Warnings()); n > 0 {
		return Result{Status: StatusWarn, Message: fmt.Sprintf("%s, %d %s", msg, n, plural(n, "warning")), Remediation: validateHint}
	}
	return Result{Status: StatusOK, Message: msg}
}

// validateHint points at the command that lists every problem with the config.
const validateHint = "run `gero config validate` for the details"

// problemCount counts the problems in a config error. The validator joins one
// error per problem, and the workspace wraps that join with the directory, so
// the count is found by walking the chain. An error that is not a join counts
// as one problem.
func problemCount(err error) int {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			return len(joined.Unwrap())
		}
	}
	return 1
}

// reposCheck looks for a local clone of each Project in the central config. It
// needs config.
type reposCheck struct{ ws Workspace }

// NewReposCheck returns the check named "repos".
func NewReposCheck(ws Workspace) Check { return reposCheck{ws: ws} }

func (reposCheck) Name() string     { return "repos" }
func (reposCheck) Category() string { return "configuration" }
func (reposCheck) Description() string {
	return "Looks for a local clone of each project in the central config"
}
func (reposCheck) Online() bool    { return false }
func (reposCheck) Needs() []string { return []string{"config"} }

func (c reposCheck) Run(context.Context) Result {
	g, err := c.ws.Chain()
	if err != nil {
		return Result{
			Status:      StatusWarn,
			Message:     "could not read the central config: " + err.Error(),
			Remediation: validateHint,
		}
	}
	var slugs []string
	for _, p := range g.Projects() {
		if !p.External() {
			slugs = append(slugs, p.Slug())
		}
	}
	if len(slugs) == 0 {
		return Result{Status: StatusOK, Message: "no projects in the central config"}
	}
	total := len(slugs)
	var unresolved []string
	var notFound, ambiguous bool
	for _, r := range c.ws.RepoPaths(slugs) {
		switch {
		case r.Err == nil:
		case errors.Is(r.Err, workspace.ErrRepoAmbiguous):
			ambiguous = true
			unresolved = append(unresolved, r.Slug+" (ambiguous)")
		default:
			notFound = true
			unresolved = append(unresolved, r.Slug+" (not found)")
		}
	}
	if len(unresolved) == 0 {
		return Result{Status: StatusOK, Message: fmt.Sprintf("%d of %d %s %s a local clone",
			total, total, plural(total, "project"), have(total))}
	}
	var advice []string
	if notFound {
		advice = append(advice, "not found: "+configresolve.HintNotFound)
	}
	if ambiguous {
		advice = append(advice, "ambiguous: "+configresolve.HintAmbiguous)
	}
	advice = append(advice, "run `gero config resolve repos` for the details")
	return Result{
		Status: StatusWarn,
		Message: fmt.Sprintf("%d of %d %s %s no unique local clone: %s",
			len(unresolved), total, plural(total, "project"), have(len(unresolved)), strings.Join(unresolved, ", ")),
		Remediation: strings.Join(advice, "; "),
	}
}

// have is the verb that agrees with a count: "has" for one, "have" otherwise.
func have(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// userFileCheck loads the user file the way every command does, so a file that
// would stop a command is reported here instead.
type userFileCheck struct{ env Environment }

// NewUserFileCheck returns the check named "user-file".
func NewUserFileCheck(env Environment) Check { return userFileCheck{env: env} }

func (userFileCheck) Name() string        { return "user-file" }
func (userFileCheck) Category() string    { return "configuration" }
func (userFileCheck) Description() string { return "Loads the user file the way every command does" }
func (userFileCheck) Online() bool        { return false }
func (userFileCheck) Needs() []string     { return nil }

func (c userFileCheck) Run(context.Context) Result {
	getenv := c.env.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	file, path, err := clicfg.OpenUserFile(getenv, c.env.Home)
	if err != nil {
		return Result{
			Status:      StatusFail,
			Message:     err.Error(),
			Remediation: "GERO_CONFIG names a file that cannot be opened: fix the path, or unset it",
		}
	}
	if file == nil {
		return Result{Status: StatusOK, Message: "no user file at " + path + " (optional)"}
	}
	defer file.Close()
	// Load the file the way every command does. The GERO_* variables are masked,
	// so a bad value in the environment is not blamed on the file.
	s, err := clicfg.Load(clicfg.Inputs{
		Getenv:       withoutGeroVars(getenv),
		UserFile:     file,
		UserFileName: path,
		Home:         c.env.Home,
	})
	if err != nil {
		return Result{
			Status:  StatusFail,
			Message: err.Error(), // the loader names the file and the key
			Remediation: "fix what the message names, or move the file aside to use the defaults; " +
				"the keys are listed in docs/specs/CLI_CONFIG.md",
		}
	}
	if len(s.Warnings) > 0 {
		// The file loads, so commands run on its valid keys; what is not known is
		// ignored, and probably a typo.
		return Result{
			Status:      StatusWarn,
			Message:     strings.Join(s.Warnings, "; "),
			Remediation: "fix or remove the keys the message names, as gero ignores them; the keys are listed in docs/specs/CLI_CONFIG.md",
		}
	}
	if missing := missingDirs(s.Repos.Search); len(missing) > 0 {
		// The scan skips a directory it cannot read without a word, so a typo hides
		// every clone under it.
		return Result{
			Status:      StatusWarn,
			Message:     "repos.search is not a directory: " + strings.Join(missing, ", "),
			Remediation: "fix or remove the repos.search entries the message names, as the scan skips them",
		}
	}
	return Result{Status: StatusOK, Message: path}
}

// missingDirs are the paths that are not an existing directory.
func missingDirs(paths []string) []string {
	var out []string
	for _, p := range paths {
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// withoutGeroVars hides the GERO_* variables from the loader, which would
// otherwise let them override, and be validated against, the file's values.
func withoutGeroVars(getenv func(string) string) func(string) string {
	return func(name string) string {
		if strings.HasPrefix(name, "GERO_") {
			return ""
		}
		return getenv(name)
	}
}
