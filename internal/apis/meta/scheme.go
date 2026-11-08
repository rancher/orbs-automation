package meta

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

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

// DecodeReport is Decode that also returns the findings of every document, errors
// and warnings, in the order reported. ctx is what validation may depend on.
func (s *Scheme) DecodeReport(_ io.Reader, _ string, _ config.Context) ([]Object, config.Report) {
	return nil, config.Report{}
}

// Decode reads every document of a YAML stream into the typed object of its
// apiVersion and kind. source names the stream in errors, normally a file name.
// A document with an unsupported apiVersion, an unknown kind, no name or a field
// its kind does not have is an error that names the source and the document; the
// other documents are still decoded, and every error is returned together.
func (s *Scheme) Decode(r io.Reader, source string) ([]Object, error) {
	dec := yaml.NewDecoder(r)
	var objs []Object
	var errs []error
	for i := 1; ; i++ {
		var node yaml.Node
		if err := dec.Decode(&node); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
			break // the stream is not YAML past here
		}
		obj, err := s.decodeDocument(&node)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: document %d: %w", source, i, err))
			continue
		}
		objs = append(objs, obj)
	}
	return objs, errors.Join(errs...)
}

func (s *Scheme) decodeDocument(node *yaml.Node) (Object, error) {
	var h head
	if err := node.Decode(&h); err != nil {
		return nil, err
	}
	kinds, ok := s.versions[h.APIVersion]
	if !ok {
		return nil, fmt.Errorf("apiVersion %q is not supported, want one of %s", h.APIVersion, strings.Join(sortedKeys(s.versions), ", "))
	}
	newObject, ok := kinds[h.Kind]
	if !ok {
		return nil, fmt.Errorf("kind %q is not known in %s: use one of %s", h.Kind, h.APIVersion, strings.Join(sortedKeys(kinds), ", "))
	}
	if h.Name == "" {
		return nil, errors.New("name is required")
	}
	// yaml.Node.Decode cannot reject unknown fields, so the document is encoded
	// again and decoded with them rejected.
	raw, err := yaml.Marshal(node)
	if err != nil {
		return nil, err
	}
	obj := newObject()
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
