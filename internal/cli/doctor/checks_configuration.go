package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/cli/configresolve"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what the configuration checks need from the invocation's
// workspace, the one door to the central config and the local clones
// (docs/specs/WORKSPACE.md). It is defined here, where it is consumed, so tests
// can use a fake; *workspace.Workspace satisfies it.
type Workspace interface {
	ConfigDir() (workspace.ConfigDir, error)
	Config() (config.Config, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// configDirCheck finds the central config directory and says where it came from.
type configDirCheck struct{ ws Workspace }

// NewConfigDirCheck returns the check named "config-dir".
func NewConfigDirCheck(ws Workspace) Check { return configDirCheck{ws: ws} }

func (configDirCheck) Name() string     { return "config-dir" }
func (configDirCheck) Category() string { return "configuration" }
func (configDirCheck) Description() string {
	return "Finds the central config directory and says where it came from"
}
func (configDirCheck) Online() bool    { return false }
func (configDirCheck) Needs() []string { return nil }

func (c configDirCheck) Run(context.Context) Result {
	dir, err := c.ws.ConfigDir()
	switch {
	case errors.Is(err, workspace.ErrNotFound):
		return Result{Status: StatusFail, Message: "no central config directory found", Remediation: workspace.NotFoundHint}
	case err != nil:
		return Result{Status: StatusFail, Message: err.Error()}
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("%s (%s)", dir.Dir, dir.Source)}
}

// configCheck loads the central application config and reports whether it is
// usable. It needs config-dir.
type configCheck struct{ ws Workspace }

// NewConfigCheck returns the check named "config".
func NewConfigCheck(ws Workspace) Check { return configCheck{ws: ws} }

func (configCheck) Name() string        { return "config" }
func (configCheck) Category() string    { return "configuration" }
func (configCheck) Description() string { return "Loads the central config and reports any problems" }
func (configCheck) Online() bool        { return false }
func (configCheck) Needs() []string     { return []string{"config-dir"} }

func (c configCheck) Run(context.Context) Result {
	cfg, err := c.ws.Config()
	if err != nil {
		n := problemCount(err)
		return Result{
			Status:      StatusFail,
			Message:     fmt.Sprintf("%d %s in the central config", n, plural(n, "problem")),
			Remediation: validateHint,
		}
	}
	lines := 0
	if cfg.Lines != nil {
		lines = len(cfg.Lines.Lines)
	}
	return Result{Status: StatusOK, Message: fmt.Sprintf("%d %s, %d %s, %d %s",
		len(cfg.Projects), plural(len(cfg.Projects), "project"),
		len(cfg.Externals), plural(len(cfg.Externals), "external"),
		lines, plural(lines, "line"))}
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
	cfg, err := c.ws.Config()
	if err != nil {
		return Result{
			Status:      StatusWarn,
			Message:     "could not read the central config: " + err.Error(),
			Remediation: validateHint,
		}
	}
	if len(cfg.Projects) == 0 {
		return Result{Status: StatusOK, Message: "no projects in the central config"}
	}
	slugs := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		slugs[i] = p.Repo.Name
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
	_, err = clicfg.Load(clicfg.Inputs{
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
	return Result{Status: StatusOK, Message: path}
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
