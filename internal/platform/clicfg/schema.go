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
	CentralConfigDir string    `yaml:"centralConfigDir"`
	ConfigSource     string    `yaml:"configSource"`
	BundleChannel    string    `yaml:"bundleChannel"`
	Output           string    `yaml:"output"`
	Color            string    `yaml:"color"`
	Upstream         string    `yaml:"upstream"`
	Fork             string    `yaml:"fork"`
	GitHubUser       string    `yaml:"githubUser"`
	BranchPrefix     string    `yaml:"branchPrefix"`
	PushTo           string    `yaml:"pushTo"`
	SignCommits      *bool     `yaml:"signCommits"`
	Repos            fileRepos `yaml:"repos"`
	Repl             fileRepl  `yaml:"repl"`
}

type fileRepl struct {
	Prompt string `yaml:"prompt"`
}

type fileRepos struct {
	// Search lists the directories gero scans for clones that already exist, to
	// depth 2, matching each to a project by its remote URL. It finds what is
	// there and puts nothing there.
	Search []string `yaml:"search"`
	// CloneRoot is the one directory gero repo clone puts new clones in, as
	// <root>/<name>. It is for clones that do not exist yet, and is never taken
	// from Search.
	CloneRoot string                      `yaml:"cloneRoot"`
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

// decodeFile parses the user file through the config engine. A key it does not
// know is not an error: the keys it does know are kept, and each unknown one is
// a warning, in warnings, so one typo does not cost the valid keys beside it.
// Anything else wrong with the file is an error. A nil reader or an empty file is
// an empty config.
func decodeFile(r io.Reader, name string) (fc fileConfig, warnings []string, err error) {
	if r == nil {
		return fc, nil, nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fileConfig{}, nil, fmt.Errorf("%s: %w", userFileLabel(name), err)
	}
	report := config.LoadTolerant(userFileLabel(name), data, &fc, config.Context{})
	if err := report.Err(); err != nil {
		return fileConfig{}, nil, err
	}
	for _, f := range report.Warnings() {
		warnings = append(warnings, f.Error())
	}
	return fc, warnings, nil
}

// UserFile is an empty value of the type the user file decodes into, so a tool
// that describes the file (tools/genschema) can read its shape without the type
// being exported.
func UserFile() any { return fileConfig{} }
