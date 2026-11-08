package v1alpha2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// kinds maps a kind to a constructor of its typed object.
var kinds = map[string]func() Object{
	KindProject:  func() Object { return new(Project) },
	KindStream:   func() Object { return new(Stream) },
	KindArtifact: func() Object { return new(Artifact) },
	KindHolding:  func() Object { return new(Holding) },
	KindEdit:     func() Object { return new(Edit) },
}

// head is the part of a document every kind shares, read before the rest is.
type head struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
}

// Decode reads every document of a YAML stream into the typed object of its
// kind. source names the stream in errors, normally a file name. A document with
// an unsupported apiVersion, an unknown kind, no name or a field its kind does
// not have is an error that names the source and the document; the other
// documents are still decoded, and every error is returned together.
func Decode(r io.Reader, source string) ([]Object, error) {
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
		obj, err := decodeDocument(&node)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: document %d: %w", source, i, err))
			continue
		}
		objs = append(objs, obj)
	}
	return objs, errors.Join(errs...)
}

func decodeDocument(node *yaml.Node) (Object, error) {
	var h head
	if err := node.Decode(&h); err != nil {
		return nil, err
	}
	if h.APIVersion != APIVersion {
		return nil, fmt.Errorf("apiVersion %q is not supported, want %q", h.APIVersion, APIVersion)
	}
	newObject, ok := kinds[h.Kind]
	if !ok {
		return nil, fmt.Errorf("kind %q is not known: use one of %s", h.Kind, strings.Join(kindNames(), ", "))
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

func kindNames() []string {
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
