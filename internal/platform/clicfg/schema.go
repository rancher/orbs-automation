package clicfg

import (
	"fmt"
	"io"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// fileConfig is the on-disk shape of the user file: a plain file of keys with no
// apiVersion or kind, unlike the versioned documents of internal/apis. Keep it in
// step with the example in docs/specs/CLI_CONFIG.md.
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
	Repl         fileRepl  `yaml:"repl"`
}

type fileRepl struct {
	Prompt string `yaml:"prompt"`
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

// SetDefaults does nothing: a key left out of the file is filled by the layers
// above it, with provenance, which one pass over the file cannot see.
func (*fileConfig) SetDefaults() {}

// Validate does nothing: the values are checked when they are resolved, with the
// layer each came from.
func (*fileConfig) Validate(config.Context, *config.Report) {}

// decodeFile parses the user file strictly through the config engine: an unknown
// key is an error. A nil reader or an empty file is an empty config.
func decodeFile(r io.Reader, name string) (fileConfig, error) {
	var fc fileConfig
	if r == nil {
		return fc, nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fileConfig{}, fmt.Errorf("%s: %w", userFileLabel(name), err)
	}
	if err := config.Load(userFileLabel(name), data, &fc, config.Context{}).Err(); err != nil {
		return fileConfig{}, err
	}
	return fc, nil
}

// UserFile is an empty value of the type the user file decodes into, so a tool
// that describes the file (tools/genschema) can read its shape without the type
// being exported.
func UserFile() any { return fileConfig{} }
