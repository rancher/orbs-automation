package chain

import (
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Graph is the whole config, joined.
type Graph struct {
	projects []*Project
	holdings []*Holding
	byName   map[string]*Project // lowercase name
	bySlug   map[string]*Project // lowercase owner/name
	lines    []lines.Line
	warnings []config.Finding
	// acknowledged are the warnings the documents say are expected.
	acknowledged []config.Finding
	empty        bool
}

// Build joins the documents of a config that has passed every rule. A document
// whose refs do not lead anywhere is not possible in such a config.
func Build(cfg appconfig.Config) *Graph { return BuildWith(cfg, config.Report{}) }

// BuildWith is Build for a caller that has the report the config was loaded with,
// whose warnings the graph keeps, and the warnings its documents acknowledge.
func BuildWith(cfg appconfig.Config, rep config.Report) *Graph {
	g := &Graph{byName: map[string]*Project{}, bySlug: map[string]*Project{}, empty: cfg.Empty(), warnings: rep.Warnings()}
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
func (g *Graph) Lines() []lines.Line { return g.lines }

// Warnings returns the warnings found when the config was loaded, in the order
// reported.
func (g *Graph) Warnings() []config.Finding { return g.warnings }

// Acknowledged returns the warnings the config's documents say are expected, in
// the order reported. They are not in Warnings.
func (g *Graph) Acknowledged() []config.Finding { return g.acknowledged }

// Empty reports whether the config has no project and no lines.
func (g *Graph) Empty() bool { return g.empty }
