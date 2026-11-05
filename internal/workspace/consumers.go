package workspace

import "github.com/rancher/orbs-automation/internal/config"

// Consumers returns the Project named by key (or the one the working directory
// is in, when key is empty) and the projects that take updates from it, each
// with the source entry that names it. The consumers may be none. It returns
// the errors Project does.
func (w *Workspace) Consumers(key string) (producer config.Project, consumers []config.Consumer, err error) {
	cfg, err := w.Config()
	if err != nil {
		return config.Project{}, nil, err
	}
	producer, err = w.projectIn(cfg, key)
	if err != nil {
		return config.Project{}, nil, err
	}
	return producer, cfg.Consumers(producer.Name), nil
}
