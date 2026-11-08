package chain

import (
	"io/fs"

	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Load reads the config under the root of fsys with the application config
// provider and joins it. The Report has every finding, errors and warnings; the
// Graph is nil when it has an error, so a caller never sees a config that failed
// a rule. The warnings are also kept on the Graph.
func Load(fsys fs.FS, ctx config.Context) (*Graph, config.Report) {
	cfg, rep := appconfig.Load(fsys, ctx)
	if rep.Err() != nil {
		return nil, rep
	}
	g := Build(cfg)
	g.warnings = rep.Warnings()
	g.empty = cfg.Empty()
	return g, rep
}

// Warnings returns the warnings found when the config was loaded, in the order
// reported.
func (g *Graph) Warnings() []config.Finding { return g.warnings }

// Empty reports whether the config has no project and no lines.
func (g *Graph) Empty() bool { return g.empty }
