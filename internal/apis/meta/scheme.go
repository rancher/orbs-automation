package meta

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// SlugPattern is what the name of a document may look like, as a regular
// expression: a to z, 0 to 9 and hyphens, starting and ending with a letter or a
// digit.
const SlugPattern = `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`

var slug = regexp.MustCompile(SlugPattern)

// Scheme knows which kinds each apiVersion has, and decodes documents into them.
type Scheme struct {
	versions map[string]map[string]func() Object
}

// NewScheme returns a Scheme with no versions.
func NewScheme() *Scheme {
	return &Scheme{versions: map[string]map[string]func() Object{}}
}

// AddKind registers a kind of an apiVersion. newObject returns a new, empty
// object of the kind for a document to be decoded into.
func (s *Scheme) AddKind(apiVersion, kind string, newObject func() Object) {
	if s.versions[apiVersion] == nil {
		s.versions[apiVersion] = map[string]func() Object{}
	}
	s.versions[apiVersion][kind] = newObject
}

// head is the part of a document every kind shares, read before the rest is.
type head struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
}

// DecodeReport reads every document of a YAML stream into the typed object of its
// apiVersion and kind, then defaults and validates each through the engine, and
// returns the objects that are valid with the findings of every document, errors
// and warnings, in the order of the documents. source names the stream in
// findings, normally a file name, and ctx is what validation may depend on.
//
// A document with an unsupported apiVersion, an unknown kind, no name, a field its
// kind does not have or a rule it breaks is dropped, and the others are still
// decoded. A YAML syntax error stops the stream.
func (s *Scheme) DecodeReport(r io.Reader, source string, ctx config.Context) ([]Object, config.Report) {
	dec := yaml.NewDecoder(r)
	var objs []Object
	var all config.Report
	for i := 1; ; i++ {
		var node yaml.Node
		if err := dec.Decode(&node); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			all.Findings = append(all.Findings, config.Finding{Source: source, Msg: err.Error()})
			break // the stream is not YAML past here
		}
		obj, rep := s.decodeDocument(source, i, &node, ctx)
		all.Findings = append(all.Findings, rep.Findings...)
		if obj != nil && rep.Err() == nil {
			objs = append(objs, obj)
		}
	}
	return objs, all
}

// Decode is DecodeReport for a caller that wants the objects and an error: the
// errors of every document joined, warnings left out, and validation as of now.
func (s *Scheme) Decode(r io.Reader, source string) ([]Object, error) {
	objs, rep := s.DecodeReport(r, source, config.Context{Now: time.Now()})
	return objs, rep.Err()
}

func (s *Scheme) decodeDocument(source string, doc int, node *yaml.Node, ctx config.Context) (Object, config.Report) {
	fail := func(format string, args ...any) (Object, config.Report) {
		return nil, config.Report{Findings: []config.Finding{{Source: source, Doc: doc, Msg: fmt.Sprintf(format, args...)}}}
	}
	var h head
	if err := node.Decode(&h); err != nil {
		return fail("%v", err)
	}
	kinds, ok := s.versions[h.APIVersion]
	if !ok {
		return fail("apiVersion %q is not supported, want one of %s", h.APIVersion, strings.Join(sortedKeys(s.versions), ", "))
	}
	newObject, ok := kinds[h.Kind]
	if !ok {
		return fail("kind %q is not known in %s: use one of %s", h.Kind, h.APIVersion, strings.Join(sortedKeys(kinds), ", "))
	}
	if h.Name == "" {
		return fail("name is required")
	}
	if !slug.MatchString(h.Name) {
		return fail("name %q must be a lowercase slug: a to z, 0 to 9 and hyphens, starting and ending with a letter or a digit", h.Name)
	}
	obj := newObject()
	return obj, config.LoadNode(source, doc, fmt.Sprintf("%s %q", h.Kind, h.Name), node, obj, ctx)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Kinds lists the kinds registered for an apiVersion, sorted. A version that is
// not registered has none.
func (s *Scheme) Kinds(apiVersion string) []string { return sortedKeys(s.versions[apiVersion]) }

// New returns a new, empty object of a kind of an apiVersion, and false when the
// scheme has no such kind.
func (s *Scheme) New(apiVersion, kind string) (Object, bool) {
	newObject, ok := s.versions[apiVersion][kind]
	if !ok {
		return nil, false
	}
	return newObject(), true
}
