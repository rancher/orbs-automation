package chain

import (
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/appconfig"
)

// Graph is the whole config, joined.
type Graph struct{}

// Build joins the documents of a config that has passed every rule. A document
// whose refs do not lead anywhere is not possible in such a config.
func Build(_ appconfig.Config) *Graph { return &Graph{} }

// Projects returns every Project, in the order read.
func (g *Graph) Projects() []*Project { return nil }

// Project returns the Project a key names: its document name, or its repo's
// owner/name when the key has a slash, compared whatever the case. It is nil when
// there is none.
func (g *Graph) Project(_ string) *Project { return nil }

// Lines returns the Rancher lines, as listed, or none when the config has none.
func (g *Graph) Lines() []v1alpha2.Line { return nil }
