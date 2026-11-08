package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rancher/orbs-automation/internal/chain"
	engine "github.com/rancher/orbs-automation/internal/platform/config"
)

// chainDir is where the v1alpha2 config is in the central config directory while
// both models exist.
const chainDir = "v1alpha2"

// Chain returns the central application config, read and joined: every document
// under the v1alpha2 folder of the config directory (see ConfigDir), validated, so
// a command never sees an invalid config. The warnings are on the graph. It
// returns ErrNotFound when there is no config directory, ErrEmptyConfig when it
// holds no document, and the provider's error, naming the file and the document,
// when the config is unusable.
func (w *Workspace) Chain() (*chain.Graph, error) {
	dir, err := w.ConfigDir()
	if err != nil {
		return nil, err
	}
	sub := filepath.Join(dir.Dir, chainDir)
	if _, err := os.Stat(sub); err != nil {
		return nil, fmt.Errorf("config directory %s: no %s folder: %w", dir.Dir, chainDir, err)
	}
	g, rep := chain.Load(os.DirFS(sub), engine.Context{Now: time.Now()})
	if err := rep.Err(); err != nil {
		return nil, fmt.Errorf("config directory %s: %w", dir.Dir, err)
	}
	if g.Empty() {
		return nil, fmt.Errorf("config directory %s: %w (the files are YAML documents in its %s folder, often a config/ subdirectory of the central checkout)", dir.Dir, ErrEmptyConfig, chainDir)
	}
	return g, nil
}

// ChainProject returns the Project of the central config named by key, or, when
// key is empty or ".", the Project whose repository contains the working
// directory, matched by its remote URLs. It returns ErrUnknownProject or
// ErrNotProject when there is no such Project, and Chain's errors when the config
// is unusable.
func (w *Workspace) ChainProject(key string) (*chain.Project, error) {
	g, err := w.Chain()
	if err != nil {
		return nil, err
	}
	if key != "" && key != "." {
		if p := g.Project(key); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("%q is %w", key, ErrUnknownProject)
	}
	here := w.cwdClone()
	if here == nil {
		return nil, ErrNotProject
	}
	var strongs, weaks []*chain.Project
	for _, p := range g.Projects() {
		switch here.matches(p.Slug()) {
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
		return nil, ErrNotProject
	}
	return found[0], nil
}
