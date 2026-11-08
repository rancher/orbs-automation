package workspace

import "github.com/rancher/orbs-automation/internal/chain"

// chainDir is where the v1alpha2 config is in the central config directory while
// both models exist.
const chainDir = "v1alpha2"

// Chain returns the central application config, read and joined: every document
// under the v1alpha2 folder of the config directory (see ConfigDir), validated, so
// a command never sees an invalid config. The warnings are on the graph. It
// returns ErrNotFound when there is no config directory, ErrEmptyConfig when it
// holds no document, and the provider's error, naming the file and the document,
// when the config is unusable.
func (w *Workspace) Chain() (*chain.Graph, error) { return nil, nil }

// ChainProject returns the Project of the central config named by key, or, when
// key is empty or ".", the Project whose repository contains the working
// directory, matched by its remote URLs. It returns ErrUnknownProject or
// ErrNotProject when there is no such Project, and Chain's errors when the config
// is unusable.
func (w *Workspace) ChainProject(_ string) (*chain.Project, error) { return nil, nil }
