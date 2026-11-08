package chain

import (
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/appconfig"
)

// Graph is the whole config, joined.
type Graph struct {
	projects []*Project
	byName   map[string]*Project // lowercase name
	bySlug   map[string]*Project // lowercase owner/name
	lines    []v1alpha2.Line
}

// Build joins the documents of a config that has passed every rule. A document
// whose refs do not lead anywhere is not possible in such a config.
func Build(cfg appconfig.Config) *Graph {
	g := &Graph{byName: map[string]*Project{}, bySlug: map[string]*Project{}}
	for _, doc := range cfg.Projects {
		p := &Project{Doc: doc, graph: g}
		g.projects = append(g.projects, p)
		g.byName[strings.ToLower(p.Name())] = p
		g.bySlug[strings.ToLower(p.Slug())] = p
	}
	for _, doc := range cfg.Streams {
		p := g.byName[strings.ToLower(doc.Refs["project"][0])]
		s := &Stream{Doc: doc, project: p}
		p.streams = append(p.streams, s)
	}
	if cfg.Lines != nil {
		g.lines = cfg.Lines.Spec.Lines
	}
	return g
}

// Projects returns every Project, in the order read.
func (g *Graph) Projects() []*Project { return g.projects }

// Project returns the Project a key names: its document name, or its repo's
// owner/name when the key has a slash, compared whatever the case. It is nil when
// there is none.
func (g *Graph) Project(key string) *Project {
	key = strings.ToLower(key)
	if strings.Contains(key, "/") {
		return g.bySlug[key]
	}
	return g.byName[key]
}

// Lines returns the Rancher lines, as listed, or none when the config has none.
func (g *Graph) Lines() []v1alpha2.Line { return g.lines }
