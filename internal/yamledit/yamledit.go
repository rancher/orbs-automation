// Package yamledit reads, and later changes, one value of a YAML file without
// disturbing the rest of it. A downstream file such as rancher's build.yaml is
// edited by a bump and then reviewed as a diff, so comments, quoting and
// indentation must survive; re-encoding the document would reflow all of them.
// It is the third importer of the YAML library, next to the two config loaders
// (docs/specs/DEPENDENCY_POLICY.md).
package yamledit

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

var (
	// ErrKeyNotFound is returned when the path names no key in the document.
	ErrKeyNotFound = errors.New("key not found")
	// ErrNotScalar is returned when the path names a mapping or a list, which
	// has no single value to read or replace.
	ErrNotScalar = errors.New("value is not a scalar")
	// ErrAmbiguous is returned when a key appears twice in the same mapping, so
	// the path does not name one value.
	ErrAmbiguous = errors.New("key appears more than once")
	// ErrUnsupported is returned for a value Set cannot replace without
	// guessing: a block scalar, a tagged or anchored value, or one that spans
	// lines. It refuses and leaves the file alone.
	ErrUnsupported = errors.New("value cannot be replaced in place")
)

// Get returns the scalar at path in the first document of src. A path is the
// keys of nested mappings joined with dots, such as global.kubectl.tag, so a key
// that itself contains a dot cannot be addressed. A quoted value comes back
// without its quotes, and a number or a boolean as its text.
func Get(src []byte, path string) (string, error) {
	node, err := find(src, path)
	if err != nil {
		return "", err
	}
	return node.Value, nil
}

// find returns the scalar node at path in the first document of src.
func find(src []byte, path string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("read YAML: %w", err)
	}
	if path == "" || len(doc.Content) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, path)
	}
	node := doc.Content[0]
	for _, key := range strings.Split(path, ".") {
		next := child(node, key)
		if next == nil {
			return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, path)
		}
		node = next
	}
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("%w: %q", ErrNotScalar, path)
	}
	return node, nil
}

// child returns the value of key in a mapping node, or nil when node is not a
// mapping or has no such key.
func child(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// Set replaces the scalar at path in the first document of src with value and
// returns the new file and the value it had. Only the bytes of the value change:
// comments, indentation, the quoting style the value had, line endings and every
// other line stay as they were, so the diff of the file is that one value. A
// plain value stays plain unless the new one needs quoting, and a quoted one
// stays quoted. A new value equal to the old one returns src unchanged. The new
// value must be one line. It returns ErrUnsupported rather than guess for a
// block scalar, a tag, an anchor or a value that spans lines.
func Set(src []byte, path, value string) (out []byte, old string, err error) {
	return src, "", nil
}
