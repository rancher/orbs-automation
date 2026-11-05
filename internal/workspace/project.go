package workspace

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
)

var (
	// ErrUnknownProject is returned when a slug names no Project of the central
	// config.
	ErrUnknownProject = errors.New("not a Project in the central config")
	// ErrNotProject is returned when no slug is given and the repository
	// containing the working directory is not a Project of the central config,
	// or there is no such repository.
	ErrNotProject = errors.New("the current repository is not a Project in the central config")
)

// Project returns the Project of the central config named by key, or, when key
// is empty, the Project whose repository
// contains the working directory. The working directory's repository is
// matched by its remote URLs, so a clone with only a fork remote still counts.
// It returns ErrUnknownProject or ErrNotProject when there is no such Project,
// and Config's errors when the central config is unusable.
func (w *Workspace) Project(key string) (config.Project, error) {
	cfg, err := w.Config()
	if err != nil {
		return config.Project{}, err
	}
	return w.projectIn(cfg, key)
}

// projectIn finds the Project named by key in cfg, or the one the working
// directory is in when key is empty.
func (w *Workspace) projectIn(cfg config.Config, key string) (config.Project, error) {
	if key != "" {
		// A key with a slash is the repository's owner/name slug, any other
		// is the document's name. Both compare case-insensitively.
		isSlug := strings.Contains(key, "/")
		for _, p := range cfg.Projects {
			if (isSlug && strings.EqualFold(p.Repo.Name, key)) || (!isSlug && strings.EqualFold(p.Name, key)) {
				return p, nil
			}
		}
		return config.Project{}, fmt.Errorf("%q is %w", key, ErrUnknownProject)
	}
	here := w.cwdClone()
	if here == nil {
		return config.Project{}, ErrNotProject
	}
	var strongs, weaks []config.Project
	for _, p := range cfg.Projects {
		switch here.matches(p.Repo.Name) {
		case strong:
			strongs = append(strongs, p)
		case weak:
			weaks = append(weaks, p)
		}
	}
	found := strongs
	if len(found) == 0 {
		found = weaks
	}
	if len(found) != 1 {
		return config.Project{}, ErrNotProject
	}
	return found[0], nil
}

// ErrEmptyConfig is returned when the config directory holds no config
// documents, which is almost always the wrong directory.
var ErrEmptyConfig = errors.New("no config documents found")
