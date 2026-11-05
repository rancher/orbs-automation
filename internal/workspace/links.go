package workspace

import "github.com/rancher/orbs-automation/internal/config"

// Links returns the Project named by key (or the one the working directory is
// in, when key is empty) and the routes out of it: each downstream edge, with
// the consumer project and the update method it names. There may be none. It
// returns the errors Project does.
func (w *Workspace) Links(key string) (producer config.Project, links []config.Link, err error) {
	cfg, err := w.Config()
	if err != nil {
		return config.Project{}, nil, err
	}
	producer, err = w.projectIn(cfg, key)
	if err != nil {
		return config.Project{}, nil, err
	}
	return producer, cfg.Links(producer), nil
}
