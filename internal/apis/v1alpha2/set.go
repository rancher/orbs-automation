package v1alpha2

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// refKey is one key of a kind's refs: which kind it points at, whether it must be
// given, and whether it takes a list of names or one.
type refKey struct {
	target   string
	required bool
	list     bool
}

// refKeys are the refs each kind has. A kind that is not here has none.
var refKeys = map[string]map[string]refKey{
	KindStream:   {"project": {target: KindProject, required: true}},
	KindArtifact: {"stream": {target: KindStream, required: true}},
	KindHolding: {
		"project":  {target: KindProject, required: true},
		"artifact": {target: KindArtifact},
		"from":     {target: KindProject},
	},
	KindEdit: {"holding": {target: KindHolding, required: true, list: true}},
}

// set is the documents of a set, indexed for the checks across them.
type set struct {
	origin func(Object) string
	ctx    config.Context
	objs   []Object
	index  map[string]map[string]Object // kind to name to the first document of it
	names  map[string]map[string]bool   // kind to the names it has
	rep    *config.Report
}

// at returns a report that names the document on its findings, and adds them to
// the set's own once reported.
func (s *set) at(o Object) *config.Report {
	source := ""
	if s.origin != nil {
		source = s.origin(o)
	}
	r := config.NewReport(source, 0, fmt.Sprintf("%s %q", o.GetKind(), o.GetName()))
	return &r
}

// add appends the findings of a report made by at.
func (s *set) add(r *config.Report) { s.rep.Findings = append(s.rep.Findings, r.Findings...) }

// ValidateSet checks what no one document can alone: the rules across a set of
// decoded and valid documents. See docs/design/CONFIG_SCHEMA.md, "Rules across
// documents".
func ValidateSet(objs []Object, ctx config.Context, r *config.Report) {
	ValidateSetFrom(objs, nil, ctx, r)
}

// ValidateSetFrom is ValidateSet for a caller that knows which source, such as a
// file, each document came from: origin names it, and a finding carries it. origin
// may be nil.
func ValidateSetFrom(objs []Object, origin func(Object) string, ctx config.Context, r *config.Report) {
	s := &set{origin: origin, ctx: ctx, objs: objs, names: map[string]map[string]bool{}, rep: r}
	s.index = map[string]map[string]Object{}
	for _, o := range objs {
		if s.index[o.GetKind()] == nil {
			s.index[o.GetKind()] = map[string]Object{}
		}
		if _, dup := s.index[o.GetKind()][o.GetName()]; !dup {
			s.index[o.GetKind()][o.GetName()] = o
		}
	}
	s.uniqueNames()
	s.refs()
	s.projects()
	s.rancherLines()
	s.knownLines()
	s.artifacts()
	s.streams()
	s.holdingRefs()
	s.holdingStages()
	s.editSteps()
	s.goBumpHoldings()
	s.circular()
	s.omittedBranches()
	s.streamsDifferFromDestination()
	s.acknowledgements()
}

// uniqueNames reports each document whose name an earlier document of its kind
// already has.
func (s *set) uniqueNames() {
	for _, o := range s.objs {
		kind := o.GetKind()
		if s.names[kind] == nil {
			s.names[kind] = map[string]bool{}
		}
		if s.names[kind][o.GetName()] {
			r := s.at(o)
			r.Errorf("name", "%s %q is defined twice", kind, o.GetName())
			s.add(r)
		}
		s.names[kind][o.GetName()] = true
	}
}

// refs checks each document's refs: each key is one its kind has, each required
// key is given, a key that takes one name has one, and every name is a document
// of the kind the key points at.
func (s *set) refs() {
	for _, o := range s.objs {
		r := s.at(o)
		keys := refKeys[o.GetKind()]
		given := o.GetRefs()
		for _, key := range sortedRefKeys(given) {
			if _, ok := keys[key]; !ok {
				r.Errorf("refs."+key, "not a ref of a %s", o.GetKind())
			}
		}
		for _, key := range sortedRefKeys(keys) {
			s.checkRef(r, key, keys[key], given[key])
		}
		s.add(r)
	}
}

// checkRef checks the names given for one key of a kind's refs.
func (s *set) checkRef(r *config.Report, key string, k refKey, names []string) {
	path := "refs." + key
	if len(names) == 0 || (len(names) == 1 && names[0] == "") {
		if k.required {
			r.Errorf(path, "required: the name of a %s", k.target)
		}
		return
	}
	if !k.list && len(names) > 1 {
		r.Errorf(path, "takes one name, got %d", len(names))
		return
	}
	for i, name := range names {
		at := path
		if k.list {
			at = fmt.Sprintf("%s[%d]", path, i)
		}
		if !s.names[k.target][name] {
			r.Errorf(at, "no %s named %q", k.target, name)
		}
	}
}

// sortedRefKeys returns the keys of a map in order, so findings come in one.
func sortedRefKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// projects reports a Project whose repo an earlier Project already has, and a
// Project that is a final destination when an earlier one already is.
func (s *set) projects() {
	repos := map[string]string{} // repo name to the Project that has it
	final := ""
	for _, o := range s.objs {
		p, ok := o.(*Project)
		if !ok {
			continue
		}
		r := s.at(o)
		key := strings.ToLower(p.Spec.Repo.Name)
		if first, dup := repos[key]; dup {
			r.Errorf("spec.repo.name", "%q is already the repo of Project %q", p.Spec.Repo.Name, first)
		} else {
			repos[key] = p.Name
		}
		if p.Spec.FinalDestination {
			if final != "" {
				r.Errorf("spec.finalDestination", "only one Project is the final destination, and it is already Project %q", final)
			} else {
				final = p.Name
			}
		}
		s.add(r)
	}
}

// knownLines reports each line a document names that RancherLines does not list.
func (s *set) knownLines() {
	known := map[string]bool{}
	for _, o := range s.objs {
		if l, ok := o.(*RancherLines); ok {
			for _, ln := range l.Spec.Lines {
				known[ln.ID] = true
			}
		}
	}
	for _, o := range s.objs {
		r := s.at(o)
		check := func(path, id string) {
			if id != "" && !known[id] {
				r.Errorf(path, "%s is not in RancherLines", id)
			}
		}
		checkSpan := func(path string, rng lines.Range) {
			check(path+".since", rng.Since)
			check(path+".until", rng.Until)
		}
		switch o := o.(type) {
		case *Project:
			for i, bl := range o.Spec.Branches.Lines {
				check(fmt.Sprintf("spec.branches.lines[%d].line", i), bl.Line)
			}
		case *Stream:
			for i, sl := range o.Spec.Lines {
				check(fmt.Sprintf("spec.lines[%d].line", i), sl.Line)
			}
		case *Artifact:
			checkSpan("spec.lines", o.Spec.Lines)
		case *Holding:
			checkSpan("spec.holds.lines", o.Spec.Holds.Lines)
		case *Edit:
			for _, steps := range []struct {
				path  string
				steps []Step
			}{{"spec.preEdit", o.Spec.PreEdit}, {"spec.postEdit", o.Spec.PostEdit}} {
				for i, st := range steps.steps {
					checkSpan(fmt.Sprintf("%s[%d]", steps.path, i), lines.Range{Since: st.Since, Until: st.Until})
				}
			}
		}
		s.add(r)
	}
}

// holdingRefs reports a Holding whose refs do not match what it holds: a release
// refs an artifact, and a commit or a branch refs the project it is taken from.
func (s *set) holdingRefs() {
	for _, o := range s.objs {
		h, ok := o.(*Holding)
		if !ok {
			continue
		}
		kind := h.Spec.Holds.Kind
		want, other := "artifact", "from"
		if kind != HoldsRelease {
			want, other = "from", "artifact"
		}
		r := s.at(o)
		if len(h.Refs[want]) == 0 {
			r.Errorf("refs."+want, "required for a holding of a %s", kind)
		}
		if len(h.Refs[other]) > 0 {
			r.Errorf("refs."+other, "not for a holding of a %s: it refs %s", kind, want)
		}
		if holder, prod := s.holder(h), s.producer(h); holder != nil && holder == prod {
			r.Errorf("refs."+want, "Project %q holds something of its own project", holder.Name)
		}
		s.add(r)
	}
}

// project returns the Project a ref names, or nil.
func (s *set) project(name string) *Project {
	p, _ := s.index[KindProject][name].(*Project)
	return p
}

// holder returns the Project a Holding is in, or nil.
func (s *set) holder(h *Holding) *Project {
	if len(h.Refs["project"]) != 1 {
		return nil
	}
	return s.project(h.Refs["project"][0])
}

// producer returns the Project a Holding holds something of: the one its
// artifact's stream is of, or the one it is taken from. It is nil when the refs
// do not lead to one.
func (s *set) producer(h *Holding) *Project {
	if names := h.Refs["from"]; len(names) == 1 {
		return s.project(names[0])
	}
	names := h.Refs["artifact"]
	if len(names) != 1 {
		return nil
	}
	a, _ := s.index[KindArtifact][names[0]].(*Artifact)
	if a == nil || len(a.Refs["stream"]) != 1 {
		return nil
	}
	st, _ := s.index[KindStream][a.Refs["stream"][0]].(*Stream)
	if st == nil || len(st.Refs["project"]) != 1 {
		return nil
	}
	return s.project(st.Refs["project"][0])
}

// circular warns, once for each pair, of two projects that each hold something
// of the other, on the holding that completes the pair.
func (s *set) circular() {
	type edge struct{ holder, producer string }
	seen := map[edge]string{} // the edge to the first Holding that makes it
	warned := map[edge]bool{}
	for _, o := range s.objs {
		h, ok := o.(*Holding)
		if !ok || len(h.Refs["project"]) != 1 {
			continue
		}
		holder := h.Refs["project"][0]
		prod := s.producer(h)
		if prod == nil || prod.Name == holder {
			continue
		}
		producer := prod.Name
		back, pair := edge{producer, holder}, edge{holder, producer}
		if first, ok := seen[back]; ok && !warned[back] && !warned[pair] {
			warned[pair] = true
			r := s.at(o)
			r.Warnf(RuleCircularHolding, "refs", "Project %q holds something of Project %q, which holds something of it (Holding %q): the dependency is circular", holder, producer, first)
			s.add(r)
		}
		if _, ok := seen[pair]; !ok {
			seen[pair] = h.Name
		}
	}
}

// omittedBranches warns of a line a Project gives no branch once the line is past
// development: an omitted branch means the default branch, which is for the line
// in development.
func (s *set) omittedBranches() {
	byID := map[string]lines.Line{}
	for _, o := range s.objs {
		if l, ok := o.(*RancherLines); ok {
			for _, ln := range l.Spec.Lines {
				byID[ln.ID] = ln
			}
		}
	}
	for _, o := range s.objs {
		p, ok := o.(*Project)
		if !ok {
			continue
		}
		r := s.at(o)
		for i, bl := range p.Spec.Branches.Lines {
			ln, known := byID[bl.Line]
			if bl.Branch != nil || !known {
				continue
			}
			if phase := ln.PhaseAt(s.ctx.Now); phase != lines.PhaseDevelopment {
				r.WarnLinef(RuleDefaultBranchPastDevelopment, bl.Line, fmt.Sprintf("spec.branches.lines[%d].branch", i), "omitted, which means the default branch, for the line in development; %s is in %s: give it a branch, or \"\" for none yet", bl.Line, phase)
			}
		}
		s.add(r)
	}
}

// rancherLines reports a RancherLines document after the first: there is one set
// of lines.
func (s *set) rancherLines() {
	first := ""
	for _, o := range s.objs {
		if _, ok := o.(*RancherLines); !ok {
			continue
		}
		if first == "" {
			first = o.GetName()
			continue
		}
		r := s.at(o)
		r.Errorf("spec", "only one RancherLines document is allowed, and %q is the first", first)
		s.add(r)
	}
}

// artifacts reports an Artifact of a kind and name that an earlier Artifact of
// the same Project has. A Go module is the exception: the same module is an
// Artifact of each minor's Stream, so for it the name is unique within a Stream.
func (s *set) artifacts() {
	type key struct{ scope, kind, name string }
	seen := map[key]string{}
	for _, o := range s.objs {
		a, ok := o.(*Artifact)
		if !ok || len(a.Refs["stream"]) != 1 {
			continue
		}
		st, _ := s.index[KindStream][a.Refs["stream"][0]].(*Stream)
		if st == nil || len(st.Refs["project"]) != 1 {
			continue
		}
		scope, scopeKind := st.Refs["project"][0], "Project"
		if a.Spec.Kind == ArtifactGoMod {
			scope, scopeKind = st.Name, "Stream"
		}
		k := key{scopeKind + " " + scope, a.Spec.Kind, a.Spec.Name}
		if first, dup := seen[k]; dup {
			r := s.at(o)
			r.Errorf("spec.name", "%q is already an artifact of %s %q, in Artifact %q", a.Spec.Name, scopeKind, scope, first)
			s.add(r)
			continue
		}
		seen[k] = a.Name
	}
}

// streams reports a calver stream whose Project does not keep every release on
// one branch: the tag carries the line, so there is no branch for it to follow.
func (s *set) streams() {
	for _, o := range s.objs {
		st, ok := o.(*Stream)
		if !ok || st.Spec.Scheme != SchemeCalverLineStage || len(st.Refs["project"]) != 1 {
			continue
		}
		p := s.project(st.Refs["project"][0])
		if p == nil || p.Spec.Branches.Layout == LayoutSingle {
			continue
		}
		r := s.at(o)
		r.Errorf("spec.scheme", "%s needs Project %q to have layout single, where every release is on the default branch, and it has %s", SchemeCalverLineStage, p.Name, p.Spec.Branches.Layout)
		s.add(r)
	}
}

// stagesOf is the names of the stages the streams of a Project have.
func (s *set) stagesOf(p *Project) map[string]bool {
	stages := map[string]bool{}
	for _, o := range s.objs {
		st, ok := o.(*Stream)
		if !ok || len(st.Refs["project"]) != 1 || st.Refs["project"][0] != p.Name {
			continue
		}
		for _, stage := range st.EffectiveStages(p.Spec.Branches.Stages) {
			stages[stage.Name] = true
		}
	}
	return stages
}

// holdingStages checks the stages a Holding names: its stage is a stage of the
// holder's branch layout, and its stages are stages the producer's streams have.
func (s *set) holdingStages() {
	for _, o := range s.objs {
		h, ok := o.(*Holding)
		if !ok {
			continue
		}
		r := s.at(o)
		holds := h.Spec.Holds
		if holder := s.holder(h); holder != nil && holds.Stage != "" {
			switch {
			case len(holder.Spec.Branches.Stages) == 0:
				r.Errorf("spec.holds.stage", "%q needs Project %q to name stages in branches.stages, and it names none", holds.Stage, holder.Name)
			case !slices.Contains(holder.Spec.Branches.Stages, holds.Stage):
				r.Errorf("spec.holds.stage", "%q is not a stage of branches.stages of Project %q (%s)", holds.Stage, holder.Name, quoteAll(holder.Spec.Branches.Stages))
			}
		}
		if prod := s.producer(h); prod != nil && len(holds.Stages) > 0 {
			have := s.stagesOf(prod)
			for i, stage := range holds.Stages {
				if stage != "" && !have[stage] {
					r.Errorf(fmt.Sprintf("spec.holds.stages[%d]", i), "%q is not a stage of Project %q", stage, prod.Name)
				}
			}
		}
		s.add(r)
	}
}

// editSteps checks that a step run from the producer has a producer that is
// owned, for each Holding the Edit follows: gero runs nothing out of a repo it
// does not ship from.
func (s *set) editSteps() {
	for _, o := range s.objs {
		e, ok := o.(*Edit)
		if !ok {
			continue
		}
		r := s.at(o)
		for _, group := range []struct {
			path  string
			steps []Step
		}{{"spec.preEdit", e.Spec.PreEdit}, {"spec.postEdit", e.Spec.PostEdit}} {
			for i, st := range group.steps {
				if st.From != FromProducer {
					continue
				}
				for _, name := range e.Refs["holding"] {
					h, _ := s.index[KindHolding][name].(*Holding)
					if h == nil {
						continue
					}
					if prod := s.producer(h); prod != nil && !prod.Spec.Repo.Owned {
						r.Errorf(fmt.Sprintf("%s[%d].from", group.path, i), "Project %q is not owned, so nothing is run from it (Holding %q)", prod.Name, name)
					}
				}
			}
		}
		s.add(r)
	}
}

// acknowledgements checks what each document says is expected, and marks the
// warnings it matches as acknowledged. An acknowledgement that matches no warning
// is a warning itself. It runs last, so every other warning exists.
func (s *set) acknowledgements() {
	for _, o := range s.objs {
		acks := o.GetAcknowledge()
		if len(acks) == 0 {
			continue
		}
		r := s.at(o)
		label := fmt.Sprintf("%s %q", o.GetKind(), o.GetName())
		for i, a := range acks {
			path := fmt.Sprintf("acknowledge[%d]", i)
			if !validAcknowledgement(r, path, a) {
				continue
			}
			if !s.acknowledge(label, a) {
				r.Warnf(RuleAcknowledgementUnused, path, "acknowledges %s, which this document does not raise%s", a.Rule, forLines(a.Lines))
			}
		}
		s.add(r)
	}
}

// acknowledge marks the warnings of a document that an acknowledgement matches:
// the same rule and, when it is limited to lines, a warning about one of them. It
// reports whether it matched any.
func (s *set) acknowledge(label string, a Acknowledgement) bool {
	matched := false
	for i := range s.rep.Findings {
		f := &s.rep.Findings[i]
		if f.Severity != config.Warning || f.Acknowledged || f.Label != label || f.Rule != a.Rule {
			continue
		}
		if a.Lines != (lines.Range{}) && (f.Line == "" || !a.Lines.Contains(f.Line)) {
			continue
		}
		f.Acknowledged, f.Reason = true, a.Reason
		matched = true
	}
	return matched
}

// Acknowledges says whether acks acknowledge a warning of a rule, about a line when
// it has one, and the reason when they do. An acknowledgement with no lines
// covers every warning of the rule; one with lines covers only a warning about a
// line in them.
func Acknowledges(_ []Acknowledgement, _, _ string) (reason string, ok bool) {
	return "", false
}

// validAcknowledgement reports an error at path for each thing wrong with an
// acknowledgement, and whether it had none.
func validAcknowledgement(r *config.Report, path string, a Acknowledgement) bool {
	before := len(r.Findings)
	switch {
	case a.Rule == "":
		r.Errorf(path+".rule", "is required")
	case !slices.Contains(acknowledgeable, a.Rule):
		r.Errorf(path+".rule", "%q is not a rule a document can acknowledge: use %s", a.Rule, orList(acknowledgeable))
	}
	if strings.TrimSpace(a.Reason) == "" {
		r.Errorf(path+".reason", "is required: say why the warning is expected")
	}
	checkRange(r, path+".lines", a.Lines)
	return len(r.Findings) == before
}

// forLines writes " for lines from v2.15 to v2.16", or nothing for every line.
func forLines(r lines.Range) string {
	switch {
	case r.Since != "" && r.Until != "":
		return " for lines " + r.Since + " to " + r.Until
	case r.Since != "":
		return " for lines from " + r.Since
	case r.Until != "":
		return " for lines up to " + r.Until
	}
	return ""
}

// streamsDifferFromDestination warns, on a holding of a project with a branch for
// each line, of a line for which the final destination holds another stream of
// the same artifact: the same kind and name, in another Stream. A line is
// compared when the holder lists it and its holding covers it, and the
// destination lists it and a holding of the same artifact covers it. The
// destination is not compared with itself, and without one nothing is compared.
func (s *set) streamsDifferFromDestination() {
	dest := s.finalDestination()
	if dest == nil {
		return
	}
	type identity struct{ kind, name string } // an artifact, by its kind and name
	type held struct {
		o       Object
		holding *Holding
		id      identity
		stream  string
	}
	destHeld := map[identity][]held{} // what the destination holds of each artifact
	var holders []held
	for _, o := range s.objs {
		h, ok := o.(*Holding)
		if !ok || h.Spec.Holds.Kind != HoldsRelease || len(h.Refs["artifact"]) != 1 {
			continue
		}
		a, _ := s.index[KindArtifact][h.Refs["artifact"][0]].(*Artifact)
		if a == nil || len(a.Refs["stream"]) != 1 {
			continue
		}
		x := held{o, h, identity{a.Spec.Kind, a.Spec.Name}, a.Refs["stream"][0]}
		if s.holder(h) == dest {
			destHeld[x.id] = append(destHeld[x.id], x)
		} else {
			holders = append(holders, x)
		}
	}
	for _, x := range holders {
		holder := s.holder(x.holding)
		if holder == nil || holder.Spec.Branches.Layout != LayoutPerLine {
			continue
		}
		r := s.at(x.o)
		for _, ln := range holder.Spec.Branches.Lines {
			if !x.holding.Spec.Holds.Lines.Contains(ln.Line) || !listsLine(dest, ln.Line) {
				continue
			}
			for _, d := range destHeld[x.id] {
				if !d.holding.Spec.Holds.Lines.Contains(ln.Line) || d.stream == x.stream {
					continue
				}
				r.WarnLinef(RuleStreamDiffersFromDestination, ln.Line, "refs.artifact",
					"holds Stream %q for %s, where Project %q, the final destination, holds Stream %q", x.stream, ln.Line, dest.Name, d.stream)
				break
			}
		}
		s.add(r)
	}
}

// finalDestination returns the Project that is the final destination, or nil.
func (s *set) finalDestination() *Project {
	for _, o := range s.objs {
		if p, ok := o.(*Project); ok && p.Spec.FinalDestination {
			return p
		}
	}
	return nil
}

// listsLine reports whether a project has a branch entry for a line.
func listsLine(p *Project, line string) bool {
	for _, bl := range p.Spec.Branches.Lines {
		if bl.Line == line {
			return true
		}
	}
	return false
}

// goBumpHoldings reports an Edit with the strategy goBump, which tidies go.mod
// files, none of whose holdings has a location in the gomod format.
func (s *set) goBumpHoldings() {
	for _, o := range s.objs {
		e, ok := o.(*Edit)
		if !ok || e.Spec.Strategy != StrategyGoBump {
			continue
		}
		found, refd := false, 0
		for _, name := range e.Refs["holding"] {
			h, _ := s.index[KindHolding][name].(*Holding)
			if h == nil {
				continue
			}
			refd++
			for _, at := range h.Spec.At {
				found = found || at.Format == FormatGoMod
			}
		}
		if refd > 0 && !found {
			r := s.at(o)
			r.Errorf("refs.holding", "%s tidies go.mod files, and none of these holdings is in the gomod format", StrategyGoBump)
			s.add(r)
		}
	}
}
