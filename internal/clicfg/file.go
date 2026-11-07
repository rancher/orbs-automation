package clicfg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// fileConfig is the on-disk shape of the user file. Keep it in step with the
// example in docs/specs/CLI_CONFIG.md.
type fileConfig struct {
	ConfigDir    string    `yaml:"configDir"`
	Output       string    `yaml:"output"`
	Color        string    `yaml:"color"`
	Upstream     string    `yaml:"upstream"`
	Fork         string    `yaml:"fork"`
	GitHubUser   string    `yaml:"githubUser"`
	BranchPrefix string    `yaml:"branchPrefix"`
	PushTo       string    `yaml:"pushTo"`
	SignCommits  *bool     `yaml:"signCommits"`
	Repos        fileRepos `yaml:"repos"`
}

type fileRepos struct {
	Search    []string                    `yaml:"search"`
	Paths     map[string]string           `yaml:"paths"`
	Overrides map[string]fileRepoOverride `yaml:"overrides"`
}

type fileRepoOverride struct {
	Upstream string `yaml:"upstream"`
	PushTo   string `yaml:"pushTo"`
}

// userFileLabel is how an error names the user file: its path, or "user config"
// when it has none.
func userFileLabel(name string) string {
	if name == "" {
		return "user config"
	}
	return name
}

// decodeFile parses the user file strictly: an unknown key is an error. A nil
// reader or an empty file is an empty config.
func decodeFile(r io.Reader, name string) (fileConfig, error) {
	var fc fileConfig
	if r == nil {
		return fc, nil
	}
	name = userFileLabel(name)
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil && !errors.Is(err, io.EOF) {
		return fileConfig{}, fmt.Errorf("%s: %w", name, err)
	}
	return fc, nil
}

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

// UserFilePath returns where the user file lives: GERO_CONFIG, else
// $XDG_CONFIG_HOME/gero/config.yaml, else ~/.config/gero/config.yaml. explicit
// is true when GERO_CONFIG named it. We avoid os.UserConfigDir because it points
// at ~/Library/Application Support on macOS.
func UserFilePath(getenv func(string) string, home string) (path string, explicit bool) {
	if p := getenv("GERO_CONFIG"); p != "" {
		return p, true
	}
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "gero", "config.yaml"), false
	}
	return filepath.Join(home, ".config", "gero", "config.yaml"), false
}

// OpenUserFile opens the user file for Load. It returns a nil reader (and no
// error) when the file does not apply: absent at the default path, or in CI
// without GERO_CONFIG. A GERO_CONFIG that names a missing file is an error,
// since it is almost certainly a typo. The caller closes a non-nil reader.
func OpenUserFile(getenv func(string) string, home string) (io.ReadCloser, string, error) {
	path, explicit := UserFilePath(getenv, home)
	ci, err := InCI(getenv)
	if err != nil {
		return nil, path, err
	}
	if ci && !explicit {
		return nil, path, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && !explicit {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("user config: %w", err)
	}
	return f, path, nil
}

// StatePath returns where gero keeps state of its own, such as the REPL's
// history, and name is the file: $XDG_STATE_HOME/gero/name, else
// ~/.local/state/gero/name. It is empty when neither is known. Like UserFilePath
// it does not use the platform's directory, which on macOS is under
// ~/Library/Application Support.
func StatePath(getenv func(string) string, home, name string) string {
	if x := getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "gero", name)
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "gero", name)
}
