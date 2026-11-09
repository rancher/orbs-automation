// Package chainload reads the application config under a file system with the
// provider and joins it into a chain.Graph. It is the one place besides the
// workspace that turns files into the graph, so a command never reads config that
// skipped the rules: the workspace is its only importer.
package chainload

import (
	"io/fs"

	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Load reads the config under the root of fsys with the application config
// provider and joins it. The Report has every finding, errors and warnings; the
// Graph is nil when it has an error, so a caller never sees a config that failed
// a rule. The warnings, and the warnings that documents acknowledge, are also kept on the Graph.
func Load(fsys fs.FS, ctx config.Context) (*chain.Graph, config.Report) {
	cfg, rep := appconfig.Load(fsys, ctx)
	if rep.Err() != nil {
		return nil, rep
	}
	return chain.BuildWith(cfg, rep), rep
}
