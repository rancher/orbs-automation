package chain

import (
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/appconfig"
)

// Graph is the whole config, joined.
type Graph struct {
	projects []*Project
	holdings []*Holding
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
	streams := map[string]*Stream{}
	for _, doc := range cfg.Streams {
		p := g.byName[strings.ToLower(doc.Refs["project"][0])]
		s := &Stream{Doc: doc, project: p}
		p.streams = append(p.streams, s)
		streams[doc.Name] = s
	}
	for _, doc := range cfg.Artifacts {
		s := streams[doc.Refs["stream"][0]]
		a := &Artifact{Doc: doc, stream: s}
		s.artifacts = append(s.artifacts, a)
		s.project.artifacts = append(s.project.artifacts, a)
	}
	artifacts := map[string]*Artifact{}
	for _, p := range g.projects {
		for _, a := range p.artifacts {
			artifacts[a.Name()] = a
		}
	}
	holdings := map[string]*Holding{}
	for _, doc := range cfg.Holdings {
		h := &Holding{Doc: doc, holder: g.byName[strings.ToLower(doc.Refs["project"][0])]}
		if names := doc.Refs["artifact"]; len(names) == 1 {
			h.artifact = artifacts[names[0]]
			h.producer = h.artifact.stream.project
		} else {
			h.producer = g.byName[strings.ToLower(doc.Refs["from"][0])]
		}
		g.holdings = append(g.holdings, h)
		holdings[doc.Name] = h
		h.holder.held = append(h.holder.held, h)
		h.producer.sourced = append(h.producer.sourced, h)
	}
	for _, doc := range cfg.Edits {
		e := &Edit{Doc: doc}
		for _, name := range doc.Refs["holding"] {
			h := holdings[name]
			e.holdings = append(e.holdings, h)
			h.edits = append(h.edits, e)
		}
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

// Holdings returns every Holding, in the order read.
func (g *Graph) Holdings() []*Holding { return g.holdings }

// Lines returns the Rancher lines, as listed, or none when the config has none.
func (g *Graph) Lines() []v1alpha2.Line { return g.lines }
