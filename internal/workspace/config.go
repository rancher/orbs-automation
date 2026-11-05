package workspace

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/config/loader"
)

// Config returns the central application config: every document in the config
// directory (see ConfigDir), with defaults applied and validated, so a command
// never sees an invalid config. It returns ErrNotFound when there is no config
// directory, and the loader's or validator's error, naming the file or
// document, when the config is unusable.
func (w *Workspace) Config() (config.Config, error) {
	dir, err := w.ConfigDir()
	if err != nil {
		return config.Config{}, err
	}
	loaded, err := loader.Load(os.DirFS(dir.Dir))
	if err != nil {
		return config.Config{}, fmt.Errorf("config directory %s: %w", dir.Dir, err)
	}
	resolved := loader.WithDefaults(loaded)
	if err := loader.Validate(resolved); err != nil {
		return config.Config{}, fmt.Errorf("config directory %s: %w", dir.Dir, err)
	}
	return resolved, nil
}
