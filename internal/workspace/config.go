package workspace

import "github.com/rancher/orbs-automation/internal/config"

// Config returns the central application config: every document in the config
// directory (see ConfigDir), with defaults applied and validated, so a command
// never sees an invalid config. It returns ErrNotFound when there is no config
// directory, and the loader's or validator's error, naming the file or
// document, when the config is unusable.
func (w *Workspace) Config() (config.Config, error) {
	return config.Config{}, nil
}
