// Package yamledit reads, and later changes, one value of a YAML file without
// disturbing the rest of it. A downstream file such as rancher's build.yaml is
// edited by a bump and then reviewed as a diff, so comments, quoting and
// indentation must survive; re-encoding the document would reflow all of them.
// It is the third importer of the YAML library, next to the two config loaders
// (docs/specs/DEPENDENCY_POLICY.md).
package yamledit

import "errors"

var (
	// ErrKeyNotFound is returned when the path names no key in the document.
	ErrKeyNotFound = errors.New("key not found")
	// ErrNotScalar is returned when the path names a mapping or a list, which
	// has no single value to read or replace.
	ErrNotScalar = errors.New("value is not a scalar")
)

// Get returns the scalar at path in the first document of src. A path is the
// keys of nested mappings joined with dots, such as global.kubectl.tag, so a key
// that itself contains a dot cannot be addressed. A quoted value comes back
// without its quotes, and a number or a boolean as its text.
func Get(src []byte, path string) (string, error) {
	return "", nil
}
