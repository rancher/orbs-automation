package clicfg

import (
	"fmt"
)

// EnvContext is the variable that forces the execution context, and ContextCI
// and ContextWorkstation are its values.
const (
	EnvContext         = "GERO_CONTEXT"
	ContextCI          = "ci"
	ContextWorkstation = "workstation"
)

// InCI reports whether the process runs in CI, where the user file is ignored
// unless GERO_CONFIG is set. It is the one place that reads CI, GITHUB_ACTIONS
// and GERO_CONTEXT (docs/specs/EXECUTION_CONTEXT.md). GERO_CONTEXT wins when
// set, and an unknown value is an error.
func InCI(getenv func(string) string) (bool, error) {
	switch v := getenv(EnvContext); v {
	case ContextCI:
		return true, nil
	case ContextWorkstation:
		return false, nil
	case "":
		return getenv("CI") == "true" || getenv("GITHUB_ACTIONS") == "true", nil
	default:
		return false, fmt.Errorf("%s must be %q or %q, got %q", EnvContext, ContextCI, ContextWorkstation, v)
	}
}

// WorkstationSigns lists the signs that a forced CI context (GERO_CONTEXT=ci)
// is really a person's machine: none of the runner variables, a TTY, and a user
// file at the default path. It returns nil when the context is not forced CI.
// tty says whether stdin and stdout are terminals, and exists whether a path is
// a file. Signs are warnings only; nothing changes what a command does
// (docs/specs/EXECUTION_CONTEXT.md).
func WorkstationSigns(getenv func(string) string, home string, tty bool, exists func(string) bool) []string {
	if getenv(EnvContext) != ContextCI {
		return nil
	}
	var signs []string
	runnerVars := []string{"GITHUB_ACTIONS", "GITHUB_RUN_ID", "GITHUB_REPOSITORY", "RUNNER_OS", "RUNNER_NAME", "RUNNER_TEMP"}
	var set bool
	for _, v := range runnerVars {
		set = set || getenv(v) != ""
	}
	if !set {
		signs = append(signs, "no GitHub Actions runner variables are set")
	}
	if tty {
		signs = append(signs, "stdin and stdout are a terminal")
	}
	if path, explicit := UserFilePath(getenv, home); !explicit && exists(path) {
		signs = append(signs, "a user file exists at "+path)
	}
	return signs
}
