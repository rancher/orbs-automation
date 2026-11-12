package chain

import (
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Graph is the whole config, joined.
type Graph struct {
	projects []*Project
	holdings []*Holding
	virtual  []*Holding          // the Holdings the pins make, in the order of the Holdings they come from
	byName   map[string]*Project // lowercase name
	bySlug   map[string]*Project // lowercase owner/name
	byAlias  map[string]*Project // lowercase alias
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
	g := &Graph{byName: map[string]*Project{}, bySlug: map[string]*Project{}, byAlias: map[string]*Project{}, empty: cfg.Empty(), warnings: rep.Warnings(), acknowledged: rep.Acknowledged()}
	for _, doc := range cfg.Projects {
		p := &Project{Doc: doc, graph: g}
		g.projects = append(g.projects, p)
		g.byName[strings.ToLower(p.Name())] = p
		g.bySlug[strings.ToLower(p.Slug())] = p
		for _, alias := range p.Aliases() {
			g.byAlias[strings.ToLower(alias)] = p
		}
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
	for _, doc := range cfg.Artifacts { // in the order read, so what pins and what is pinned come in one
		a := artifacts[doc.Name]
		for _, name := range doc.Refs["pins"] {
			a.pins = append(a.pins, artifacts[name])
			artifacts[name].pinnedBy = append(artifacts[name].pinnedBy, a)
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
	g.makeVirtual()
	for _, doc := range cfg.Edits {
		e := &Edit{Doc: doc}
		for _, name := range doc.Refs["holding"] {
			h := holdings[name]
			e.holdings = append(e.holdings, h)
			h.edits = append(h.edits, e)
		}
		for _, name := range doc.Refs["holding"] {
			for _, v := range holdings[name].pinned {
				e.holdings = append(e.holdings, v) // the file a pin writes is part of what the edit follows
				v.edits = append(v.edits, e)
			}
		}
	}
	if cfg.Lines != nil {
		g.lines = cfg.Lines.Spec.Lines
	}
	return g
}

// makeVirtual makes the Holdings the pins make: for each Holding of an Artifact
// that pins others, one of each pinned Artifact, in the same files and for the same
// lines, held in the pinned module's own path as a Go module is, as a floor. Each
// is a Holding of its holder and a source of the pinned Artifact's project, so a
// route, a check and a run see it as they see any other.
func (g *Graph) makeVirtual() {
	for _, h := range g.holdings {
		if h.artifact == nil {
			continue
		}
		for _, pinned := range h.artifact.pins {
			doc := &v1alpha2.Holding{
				TypeMeta: h.Doc.TypeMeta,
				ObjectMeta: v1alpha2.ObjectMeta{
					Name: h.Name() + "+" + pinned.Name(),
					Refs: v1alpha2.Refs{"project": h.Doc.Refs["project"], "artifact": {pinned.Name()}},
				},
				Spec: v1alpha2.HoldingSpec{
					Holds: v1alpha2.Holds{Kind: v1alpha2.HoldsRelease, Form: "{bareTag}", Lines: h.Doc.Spec.Holds.Lines, Accept: v1alpha2.AcceptAll},
				},
			}
			for _, at := range h.Doc.Spec.At {
				doc.Spec.At = append(doc.Spec.At, v1alpha2.At{File: at.File, Key: pinned.Ref(), Format: v1alpha2.FormatGoMod, Lines: at.Lines})
			}
			v := &Holding{
				Doc: doc, holder: h.holder, producer: pinned.stream.project, artifact: pinned,
				virtual: true, pinnedBy: h,
			}
			h.pinned = append(h.pinned, v)
			g.virtual = append(g.virtual, v)
			v.holder.held = append(v.holder.held, v)
			v.producer.sourced = append(v.producer.sourced, v)
		}
	}
}

// Projects returns every Project, in the order read.
func (g *Graph) Projects() []*Project { return g.projects }

// Project returns the Project a key names: its document name or one of its
// aliases, or its repo's owner/name when the key has a slash, compared whatever
// the case. It is nil when there is none.
func (g *Graph) Project(key string) *Project {
	key = strings.ToLower(key)
	if strings.Contains(key, "/") {
		return g.bySlug[key]
	}
	if p := g.byName[key]; p != nil {
		return p
	}
	return g.byAlias[key]
}

// Virtual returns the Holdings the pins make, in the order of the Holdings they
// come from. They are no documents, so Holdings does not list them.
func (g *Graph) Virtual() []*Holding { return g.virtual }

// Holdings returns every Holding, in the order read.
func (g *Graph) Holdings() []*Holding { return g.holdings }

// Lines returns the Rancher lines, as listed, or none when the config has none.
func (g *Graph) Lines() []lines.Line { return g.lines }

// Line returns the Rancher line of the lines document that an id names, written
// with or without its v, in any case; ok is false when there is none.
func (g *Graph) Line(id string) (line lines.Line, ok bool) {
	if id == "" {
		return lines.Line{}, false
	}
	i := slices.IndexFunc(g.lines, func(l lines.Line) bool { return lines.Normalize(l.ID) == lines.Normalize(id) })
	if i < 0 {
		return lines.Line{}, false
	}
	return g.lines[i], true
}

// Warnings returns the warnings found when the config was loaded, in the order
// reported.
func (g *Graph) Warnings() []config.Finding { return g.warnings }

// Acknowledged returns the warnings the config's documents say are expected, in
// the order reported. They are not in Warnings.
func (g *Graph) Acknowledged() []config.Finding { return g.acknowledged }

// Empty reports whether the config has no project and no lines.
func (g *Graph) Empty() bool { return g.empty }
