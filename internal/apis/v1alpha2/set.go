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
