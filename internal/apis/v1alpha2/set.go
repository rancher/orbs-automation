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
func ValidateSet(objs []Object, _ config.Context, r *config.Report) {
	s := &set{objs: objs, names: map[string]map[string]bool{}, rep: r}
	s.uniqueNames()
	s.refs()
	s.projects()
	s.knownLines()
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
