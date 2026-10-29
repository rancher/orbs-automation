package clicfg

import (
	"errors"
	"fmt"
	"io"

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

// decodeFile parses the user file strictly: an unknown key is an error. A nil
// reader or an empty file is an empty config.
func decodeFile(r io.Reader, name string) (fileConfig, error) {
	var fc fileConfig
	if r == nil {
		return fc, nil
	}
	if name == "" {
		name = "user config"
	}
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil && !errors.Is(err, io.EOF) {
		return fileConfig{}, fmt.Errorf("%s: %w", name, err)
	}
	return fc, nil
}
