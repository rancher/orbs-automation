package v1alpha2

import (
	"fmt"
	"sort"

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
	ctx   config.Context
	objs  []Object
	names map[string]map[string]bool // kind to the names it has
	rep   *config.Report
}

// at returns a report that names the document on its findings, and adds them to
// the set's own once reported.
func (s *set) at(o Object) *config.Report {
	r := config.NewReport("", 0, fmt.Sprintf("%s %q", o.GetKind(), o.GetName()))
	return &r
}

// add appends the findings of a report made by at.
func (s *set) add(r *config.Report) { s.rep.Findings = append(s.rep.Findings, r.Findings...) }

// ValidateSet checks what no one document can alone: the rules across a set of
// decoded and valid documents. See docs/design/CONFIG_SCHEMA.md, "Rules across
// documents".
func ValidateSet(objs []Object, ctx config.Context, r *config.Report) {
	s := &set{ctx: ctx, objs: objs, names: map[string]map[string]bool{}, rep: r}
	s.uniqueNames()
	s.refs()
	s.projects()
	s.knownLines()
	s.holdingRefs()
	s.circular()
	s.omittedBranches()
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
		if first, dup := repos[p.Spec.Repo.Name]; dup {
			r.Errorf("spec.repo.name", "%q is already the repo of Project %q", p.Spec.Repo.Name, first)
		} else {
			repos[p.Spec.Repo.Name] = p.Name
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
		checkSpan := func(path string, rng Range) {
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
					checkSpan(fmt.Sprintf("%s[%d]", steps.path, i), Range{Since: st.Since, Until: st.Until})
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
		s.add(r)
	}
}

// producer returns the name of the Project a Holding holds something of: the one
// its artifact's stream is of, or the one it is taken from. It is empty when the
// refs do not lead to one.
func (s *set) producer(h *Holding) string {
	if names := h.Refs["from"]; len(names) == 1 {
		return names[0]
	}
	names := h.Refs["artifact"]
	if len(names) != 1 {
		return ""
	}
	for _, o := range s.objs {
		if a, ok := o.(*Artifact); ok && a.Name == names[0] {
			for _, o := range s.objs {
				if st, ok := o.(*Stream); ok && len(a.Refs["stream"]) == 1 && st.Name == a.Refs["stream"][0] && len(st.Refs["project"]) == 1 {
					return st.Refs["project"][0]
				}
			}
		}
	}
	return ""
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
		holder, producer := h.Refs["project"][0], s.producer(h)
		if producer == "" || producer == holder {
			continue
		}
		back, pair := edge{producer, holder}, edge{holder, producer}
		if first, ok := seen[back]; ok && !warned[back] && !warned[pair] {
			warned[pair] = true
			r := s.at(o)
			r.Warnf("refs", "Project %q holds something of Project %q, which holds something of it (Holding %q): the dependency is circular", holder, producer, first)
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
	lines := map[string]Line{}
	for _, o := range s.objs {
		if l, ok := o.(*RancherLines); ok {
			for _, ln := range l.Spec.Lines {
				lines[ln.ID] = ln
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
			ln, known := lines[bl.Line]
			if bl.Branch != nil || !known {
				continue
			}
			if phase := ln.PhaseAt(s.ctx.Now); phase != PhaseDevelopment {
				r.Warnf(fmt.Sprintf("spec.branches.lines[%d].branch", i), "omitted, which means the default branch, for the line in development; %s is in %s: give it a branch, or \"\" for none yet", bl.Line, phase)
			}
		}
		s.add(r)
	}
}
