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
	Upstream     string    `yaml:"upstream"`
	Fork         string    `yaml:"fork"`
	GitHubUser   string    `yaml:"githubUser"`
	BranchPrefix string    `yaml:"branchPrefix"`
	Repos        fileRepos `yaml:"repos"`
}

type fileRepos struct {
	Search    []string                    `yaml:"search"`
	Paths     map[string]string           `yaml:"paths"`
	Overrides map[string]fileRepoOverride `yaml:"overrides"`
}

type fileRepoOverride struct {
	Upstream string `yaml:"upstream"`
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

// Values of GERO_CONTEXT, which forces the execution context.
const (
	ContextCI          = "ci"
	ContextWorkstation = "workstation"
)

// InCI reports whether the process runs in CI, where the user file is ignored
// unless GERO_CONFIG is set. It is the one place that reads CI, GITHUB_ACTIONS
// and GERO_CONTEXT (docs/specs/EXECUTION_CONTEXT.md). An unknown GERO_CONTEXT
// is an error.
func InCI(getenv func(string) string) (bool, error) {
	return getenv("CI") == "true" || getenv("GITHUB_ACTIONS") == "true", nil
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
