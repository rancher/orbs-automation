package workspace

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chainload"
	engine "github.com/rancher/orbs-automation/internal/platform/config"
)

var (
	// ErrUnknownProject is returned when a slug names no Project of the central
	// config.
	ErrUnknownProject = errors.New("not a Project in the central config")
	// ErrNotProject is returned when no slug is given and the repository
	// containing the working directory is not a Project of the central config,
	// or there is no such repository.
	ErrNotProject = errors.New("the current repository is not a Project in the central config")
	// ErrEmptyConfig is returned when the config directory holds no config
	// documents, which is almost always the wrong directory.
	ErrEmptyConfig = errors.New("no config documents found")
)

// Chain returns the central application config, read and joined: every document
// of every YAML file under the config directory (see CentralConfigDir), validated, so a
// command never sees an invalid config. The warnings are on the graph. It is read
// once for the invocation, so every caller has the same graph. It returns ErrNotFound when there is no config directory, ErrEmptyConfig when it
// holds no document, and the provider's error, naming the file and the document,
// when the config is unusable.
func (w *Workspace) Chain() (*chain.Graph, error) {
	w.chainOnce.Do(func() { w.graph, w.graphErr = w.loadChain() })
	return w.graph, w.graphErr
}

// loadChain reads and joins the config once, from the local directory or, when
// none is named, from the bundle (ConfigSource).
func (w *Workspace) loadChain() (*chain.Graph, error) {
	src, err := w.ConfigSource()
	if err != nil {
		return nil, err
	}
	if src.Bundle != nil {
		g, rep := chainload.Load(w.bundleFiles.FS, engine.Context{Now: time.Now()})
		if err := rep.Err(); err != nil {
			return nil, fmt.Errorf("config bundle %s: %w", src.Bundle.Tag, err)
		}
		if g.Empty() {
			return nil, fmt.Errorf("config bundle %s: %w", src.Bundle.Tag, ErrEmptyConfig)
		}
		return g, nil
	}
	dir := src.Dir
	g, rep := chainload.Load(os.DirFS(dir.Dir), engine.Context{Now: time.Now()})
	if err := rep.Err(); err != nil {
		return nil, fmt.Errorf("config directory %s: %w", dir.Dir, err)
	}
	if g.Empty() {
		return nil, fmt.Errorf("config directory %s: %w (the files are YAML documents of gero's, often in the config/ subdirectory of the central checkout)", dir.Dir, ErrEmptyConfig)
	}
	return g, nil
}

// Project returns the Project of the central config named by key, or, when
// key is empty or ".", the Project whose repository contains the working
// directory, matched by its remote URLs. It returns ErrUnknownProject or
// ErrNotProject when there is no such Project, and Chain's errors when the config
// is unusable.
func (w *Workspace) Project(key string) (*chain.Project, error) {
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
